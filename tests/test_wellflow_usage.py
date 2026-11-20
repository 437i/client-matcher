import contextlib
from decimal import Decimal
import io
import json
import os
from pathlib import Path
import stat
import tempfile
import unittest
from unittest.mock import Mock, patch
from urllib.error import HTTPError

from scripts import wellflow_usage as usage


KEY = 'synthetic-secret-not-a-real-key'


def record(**overrides):
    result = dict(created_at='2026-11-20T10:00:00Z', model='synthetic-model',
                  input_tokens=2, output_tokens=3, total_tokens=5, cost_cents='0.00125')
    result.update(overrides)
    return result


def response(rows=None, cursor=None):
    return json.dumps(dict(data=[] if rows is None else rows, next_cursor=cursor)).encode()


def opener_for(*results):
    opener = Mock()
    effects = []
    for result in results:
        if isinstance(result, Exception):
            effects.append(result)
        else:
            handle = contextlib.nullcontext(io.BytesIO(result))
            effects.append(handle)
    opener.open.side_effect = effects
    return opener


def http_error(code, headers=None):
    return HTTPError(usage.API, code, KEY, headers or {}, io.BytesIO(KEY.encode()))


class CollectorTests(unittest.TestCase):
    def test_pagination_and_duplicates(self):
        row = record()
        opener = opener_for(response([row], 'next & page'), response([row]))
        rows = usage.collect(KEY, opener)
        self.assertEqual(rows, [row, row])
        self.assertEqual(usage.summarize(rows), {'synthetic-model': '0.00250'})
        first, second = [call.args[0] for call in opener.open.call_args_list]
        self.assertEqual(first.full_url, usage.API + '?limit=20')
        self.assertIn('cursor=next+%26+page', second.full_url)
        self.assertEqual(first.get_header('Authorization'), 'Bearer ' + KEY)
        self.assertEqual(opener.open.call_args.kwargs['timeout'], 30)

    def test_empty_history(self):
        self.assertEqual(usage.collect(KEY, opener_for(response())), [])
        self.assertEqual(usage.summarize([]), {})

    def test_fractional_json_number_and_large_precision(self):
        raw = response([record()]).replace(b'"0.00125"', b'0.0000000000000000000000000000000125')
        rows = usage.collect(KEY, opener_for(raw))
        self.assertEqual(rows[0]['cost_cents'], Decimal('1.25e-32'))
        self.assertEqual(Decimal(usage.summarize(rows * 2)['synthetic-model']), Decimal('2.50e-32'))

    def test_repeat_cursor(self):
        with self.assertRaisesRegex(usage.UsageError, 'Repeated'):
            usage.collect(KEY, opener_for(response([], 'same'), response([], 'same')))

    def test_malformed_json(self):
        for raw in (b'{', b'not json', bytes([255]), b'{"data":NaN}'):
            with self.subTest(raw=raw), self.assertRaises(usage.UsageError):
                usage.collect(KEY, opener_for(raw))

    def test_invalid_types(self):
        pages = [[], {}, {'data': {}, 'next_cursor': None}, {'data': [], 'next_cursor': 1},
                 {'data': [], 'next_cursor': ''}]
        rows = [None, record(model=None), record(model=''), record(created_at=1),
                record(created_at='2026-11-20'), record(input_tokens=True),
                record(input_tokens='2'), record(output_tokens=-1), record(total_tokens=None),
                record(cost_cents=True), record(cost_cents=None), record(cost_cents=[]),
                record(cost_cents='NaN'), record(cost_cents='Infinity'), record(cost_cents='bad'),
                record(cost_cents='-1')]
        pages += [dict(data=[row], next_cursor=None) for row in rows]
        for page in pages:
            with self.subTest(page=page), self.assertRaises(usage.UsageError):
                usage.decode_page(json.dumps(page).encode())

    def test_timeout_bounded_and_recovery(self):
        sleep = Mock()
        opener = opener_for(TimeoutError(KEY), TimeoutError(KEY), TimeoutError(KEY))
        with self.assertRaises(usage.UsageError) as caught:
            usage.collect(KEY, opener, sleep)
        self.assertNotIn(KEY, str(caught.exception))
        self.assertEqual(opener.open.call_count, 3)
        self.assertEqual(sleep.call_count, 2)
        self.assertEqual(usage.collect(KEY, opener_for(TimeoutError(), response()), sleep), [])

    def test_401_403_no_retry(self):
        for code in (401, 403):
            opener = opener_for(http_error(code))
            with self.subTest(code=code), self.assertRaisesRegex(usage.UsageError, str(code)):
                usage.collect(KEY, opener, Mock())
            self.assertEqual(opener.open.call_count, 1)

    def test_retry_429_5xx(self):
        sleep = Mock()
        opener = opener_for(http_error(429, {'Retry-After': '999'}), http_error(503), response())
        self.assertEqual(usage.collect(KEY, opener, sleep), [])
        self.assertEqual(sleep.call_args_list[0].args, (30,))
        for code in (429, 500, 599):
            opener = opener_for(*(http_error(code) for _ in range(3)))
            with self.subTest(code=code), self.assertRaises(usage.UsageError):
                usage.collect(KEY, opener, Mock())
            self.assertEqual(opener.open.call_count, 3)

    def test_key_environment_precedence(self):
        with patch.dict(os.environ, {'WELLFLOW_API_KEY': KEY}, clear=True), \
                patch.object(Path, 'read_text', side_effect=AssertionError('must not read auth')):
            self.assertEqual(usage.load_key(), KEY)

    def test_key_synthetic_auth_and_missing(self):
        with tempfile.TemporaryDirectory() as directory:
            auth = Path(directory) / 'auth.json'
            with patch.dict(os.environ, {'CODEX_HOME': directory}, clear=True):
                with self.assertRaises(usage.UsageError):
                    usage.load_key()
                auth.write_text(json.dumps({'OPENAI_API_KEY': KEY}))
                self.assertEqual(usage.load_key(), KEY)
                for content in ('{', '[]', '{}', '{"OPENAI_API_KEY":1}'):
                    auth.write_text(content)
                    with self.assertRaises(usage.UsageError):
                        usage.load_key()
            with patch.dict(os.environ, {}, clear=True), patch.object(Path, 'home', return_value=Path(directory)):
                (Path(directory) / '.codex').mkdir()
                (Path(directory) / '.codex/auth.json').write_text(json.dumps({'OPENAI_API_KEY': KEY}))
                self.assertEqual(usage.load_key(), KEY)

    def test_no_key_or_error_leak_in_output(self):
        output = io.StringIO()
        with patch.object(usage, 'load_key', return_value=KEY), \
                patch.object(usage, 'collect', side_effect=usage.UsageError(KEY)), \
                patch.object(usage, 'save_snapshot') as save, \
                contextlib.redirect_stdout(output), contextlib.redirect_stderr(output):
            self.assertEqual(usage.main([]), 1)
            save.assert_not_called()
        self.assertNotIn(KEY, output.getvalue())
        with self.assertRaises(usage.UsageError):
            usage.collect(KEY, opener_for(response([record(model=KEY)])))

    def test_snapshot_precision_permissions_and_duplicates(self):
        with tempfile.TemporaryDirectory() as directory:
            rows = [record(cost_cents=Decimal('0.00000000000000000000125'))] * 2
            first = usage.save_snapshot(rows, directory)
            second = usage.save_snapshot(rows, directory)
            self.assertNotEqual(first, second)
            snapshot = json.loads(first.read_text())
            self.assertTrue(snapshot['complete'])
            self.assertEqual(len(snapshot['data']), 2)
            self.assertEqual(Decimal(snapshot['data'][0]['cost_cents']), rows[0]['cost_cents'])
            self.assertNotIn(KEY, first.read_text())
            self.assertEqual(stat.S_IMODE(first.stat().st_mode), 0o600)
            self.assertEqual(stat.S_IMODE(Path(directory).stat().st_mode), 0o700)
            self.assertEqual(len(list(Path(directory).iterdir())), 2)

    def test_atomic_failure_does_not_publish(self):
        with tempfile.TemporaryDirectory() as directory, \
                patch.object(usage.os, 'replace', side_effect=OSError('synthetic')):
            with self.assertRaises(OSError):
                usage.save_snapshot([], directory)
            self.assertEqual(list(Path(directory).iterdir()), [])

    def test_failure_on_later_page_does_not_save(self):
        opener = opener_for(response([record()], 'next'), b'{')
        with patch.object(usage, 'load_key', return_value=KEY), \
                patch.object(usage, 'build_opener', return_value=opener), \
                patch.object(usage, 'save_snapshot') as save, contextlib.redirect_stderr(io.StringIO()):
            self.assertEqual(usage.main([]), 1)
            save.assert_not_called()

    def test_interval_and_models(self):
        rows = [record(created_at='2026-11-19T10:00:00Z'), record(),
                record(model='other'), record(created_at='2026-11-21T00:00:00Z')]
        totals = usage.summarize(rows, usage.timestamp('2026-11-20T10:00:00Z'),
                                 usage.timestamp('2026-11-21T00:00:00Z'))
        self.assertEqual(totals, {'other': '0.00125', 'synthetic-model': '0.00125'})

    def test_redirect_refused(self):
        with self.assertRaises(usage.UsageError):
            usage.NoRedirect().redirect_request(None, None, 302, '', {}, 'https://elsewhere.invalid')

    def test_successful_main_filters_report_but_saves_full_history(self):
        rows = [record(created_at='2026-11-19T10:00:00Z'), record()]
        output = io.StringIO()
        with tempfile.TemporaryDirectory() as directory, \
                patch.object(usage, 'SNAPSHOT_DIR', Path(directory)), \
                patch.object(usage, 'load_key', return_value=KEY), \
                patch.object(usage, 'build_opener', return_value=opener_for(response(rows))), \
                contextlib.redirect_stdout(output), contextlib.redirect_stderr(output):
            self.assertEqual(usage.main(['--from', '2026-11-20T00:00:00Z']), 0)
            snapshots = list(Path(directory).glob('*.json'))
            self.assertEqual(len(snapshots), 1)
            self.assertEqual(json.loads(snapshots[0].read_text())['data'], rows)
            self.assertNotIn(KEY, snapshots[0].read_text())
        self.assertIn('0.00125', output.getvalue())
        self.assertNotIn(KEY, output.getvalue())

    def test_missing_key_main_does_not_call_network(self):
        with patch.object(usage, 'load_key', side_effect=usage.UsageError('API key unavailable')), \
                patch.object(usage, 'collect') as collect, contextlib.redirect_stderr(io.StringIO()):
            self.assertEqual(usage.main([]), 1)
            collect.assert_not_called()


if __name__ == '__main__':
    unittest.main()
