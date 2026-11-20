#!/usr/bin/env python3
"""Collect complete Wellflow usage locally; never print credentials."""
import argparse
from datetime import datetime, timezone
from decimal import Decimal, InvalidOperation, localcontext
from email.utils import parsedate_to_datetime
import json
import math
import os
from pathlib import Path
import socket
import sys
import tempfile
import time
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import HTTPRedirectHandler, Request, build_opener


API = 'https://api.wellflow.dev/v1/usage'
SNAPSHOT_DIR = Path.home() / 'Library/Application Support/client-matcher/wellflow-usage'


class UsageError(Exception):
    pass


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise UsageError('API redirect refused')


def load_key():
    value = os.environ.get('WELLFLOW_API_KEY')
    if value is None:
        directory = Path(os.environ.get('CODEX_HOME', str(Path.home() / '.codex')))
        try:
            auth = json.loads((directory / 'auth.json').read_text())
            value = auth.get('OPENAI_API_KEY') if isinstance(auth, dict) else None
        except (OSError, ValueError):
            raise UsageError('API key unavailable') from None
    if not isinstance(value, str) or not value.strip() or any(c.isspace() for c in value):
        raise UsageError('API key unavailable or invalid')
    return value


def timestamp(value):
    if not isinstance(value, str):
        raise UsageError('created_at must be an ISO timestamp')
    try:
        result = datetime.fromisoformat(value.replace('Z', '+00:00'))
    except ValueError:
        raise UsageError('Invalid timestamp') from None
    if result.tzinfo is None:
        raise UsageError('Timestamp requires timezone')
    return result


def reject_constant(value):
    raise ValueError('Non-finite JSON number')


def decode_page(raw):
    try:
        page = json.loads(raw, parse_float=Decimal, parse_constant=reject_constant)
    except (ValueError, UnicodeError):
        raise UsageError('Invalid JSON response') from None
    if not isinstance(page, dict) or not isinstance(page.get('data'), list) or 'next_cursor' not in page:
        raise UsageError('Invalid page structure')
    cursor = page['next_cursor']
    if cursor is not None and (not isinstance(cursor, str) or not cursor):
        raise UsageError('Invalid next_cursor')
    for row in page['data']:
        if not isinstance(row, dict):
            raise UsageError('Invalid usage record')
        timestamp(row.get('created_at'))
        if not isinstance(row.get('model'), str) or not row['model'].strip():
            raise UsageError('Invalid model')
        for field in ('input_tokens', 'output_tokens', 'total_tokens'):
            if type(row.get(field)) is not int or row[field] < 0:
                raise UsageError('Invalid token count')
        cost = row.get('cost_cents')
        if isinstance(cost, bool) or not isinstance(cost, (str, int, Decimal)):
            raise UsageError('Invalid cost_cents')
        try:
            amount = Decimal(cost)
        except InvalidOperation:
            raise UsageError('Invalid cost_cents') from None
        if not amount.is_finite() or amount < 0:
            raise UsageError('Invalid cost_cents')
    return page


def fetch_page(url, key, opener, sleep=time.sleep, attempts=3):
    for attempt in range(attempts):
        delay = min(2 ** attempt, 10)
        try:
            request = Request(url, headers={'Authorization': 'Bearer ' + key, 'Accept': 'application/json'})
            with opener.open(request, timeout=30) as response:
                return decode_page(response.read())
        except HTTPError as error:
            status = error.code
            retry_after = error.headers.get('Retry-After') if error.headers else None
            error.close()
            if status in (401, 403):
                raise UsageError('Authorization refused (HTTP %s)' % status) from None
            if status != 429 and not 500 <= status <= 599:
                raise UsageError('API request failed (HTTP %s)' % status) from None
            if retry_after:
                try:
                    seconds = float(retry_after)
                    if math.isfinite(seconds):
                        delay = min(max(seconds, 0), 30)
                except ValueError:
                    try:
                        seconds = (parsedate_to_datetime(retry_after) - datetime.now(timezone.utc)).total_seconds()
                        delay = min(max(seconds, 0), 30)
                    except (ValueError, TypeError, OverflowError):
                        pass
        except (TimeoutError, socket.timeout, URLError, OSError):
            pass
        if attempt + 1 == attempts:
            raise UsageError('API request failed after bounded retries') from None
        sleep(delay)
    raise UsageError('No request attempts')


def collect(key, opener=None, sleep=time.sleep):
    opener = opener if opener is not None else build_opener(NoRedirect())
    records, seen, cursor = [], set(), None
    while True:
        query = {'limit': 20}
        if cursor is not None:
            query['cursor'] = cursor
        page = fetch_page(API + '?' + urlencode(query), key, opener, sleep)
        # Refuse unexpected credential echoes, including in unknown response fields.
        if key in json.dumps(page, default=str):
            raise UsageError('Response contains credential material')
        records.extend(page['data'])
        cursor = page['next_cursor']
        if cursor is None:
            return records
        if cursor in seen:
            raise UsageError('Repeated pagination cursor')
        seen.add(cursor)


def summarize(records, start=None, until=None):
    totals = {}
    for row in records:
        created = timestamp(row['created_at'])
        if (start is not None and created < start) or (until is not None and created >= until):
            continue
        cost = Decimal(row['cost_cents'])
        previous = totals.get(row['model'], Decimal(0))
        # Decimal's default precision is 28; grow it to preserve arbitrary input fractions.
        with localcontext() as context:
            context.prec = max(previous.adjusted(), cost.adjusted(), 0) - min(previous.as_tuple().exponent, cost.as_tuple().exponent, 0) + 3
            totals[row['model']] = previous + cost
    return {model: str(cost) for model, cost in sorted(totals.items())}


def save_snapshot(records, directory=None):
    directory = SNAPSHOT_DIR if directory is None else Path(directory)
    directory = directory.expanduser().resolve()
    # The default lives outside the checkout; refuse accidental output into any Git worktree.
    if any((parent / '.git').exists() for parent in (directory, *directory.parents)):
        raise UsageError('Snapshot directory must be outside Git')
    directory.mkdir(parents=True, exist_ok=True, mode=0o700)
    if sys.platform == 'darwin':
        directory.chmod(0o700)
    descriptor, temporary = tempfile.mkstemp(prefix='.usage-', suffix='.tmp', dir=directory)
    target = directory / (Path(temporary).name[1:-4] + '.json')
    try:
        os.fchmod(descriptor, 0o600)
        with os.fdopen(descriptor, 'w', encoding='utf-8') as output:
            json.dump({'complete': True, 'collected_at': datetime.now(timezone.utc).isoformat(),
                       'cost_unit': 'cost_cents (unverified)', 'data': records},
                      output, ensure_ascii=False, default=str, allow_nan=False, indent=2)
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, target)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)
    return target


def main(argv=None):
    parser = argparse.ArgumentParser(description='Collect complete Wellflow history; costs remain raw cost_cents.')
    parser.add_argument('--from', dest='start', help='Inclusive ISO timestamp with timezone')
    parser.add_argument('--until', help='Exclusive ISO timestamp with timezone')
    arguments = parser.parse_args(argv)
    try:
        start = timestamp(arguments.start) if arguments.start else None
        until = timestamp(arguments.until) if arguments.until else None
        if start and until and start >= until:
            raise UsageError('Interval requires from < until')
        key = load_key()
        records = collect(key)
        totals = summarize(records, start, until)
        snapshot = save_snapshot(records)
        print('Snapshot:', snapshot)
        print('Raw cost_cents (unit unverified):', json.dumps(totals, ensure_ascii=False))
        print('No task attribution or invented request IDs; snapshot contains the full history.')
        return 0
    except UsageError as error:
        # UsageError messages are fixed local diagnostics; never include server bodies.
        message = str(error)
        if 'key' in locals():
            message = message.replace(key, '[redacted]')
        print('Collection failed: ' + message, file=sys.stderr)
        return 1
    except (OSError, ValueError, InvalidOperation):
        print('Collection failed: local storage or numeric error.', file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
