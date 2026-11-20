#!/usr/bin/env python3
"""Finish the one-time skill move locally after inspecting --check output."""
import argparse
import os
from pathlib import Path


NAMES = ('safe-go-change', 'backend-review', 'openapi-swagger',
         'frontend-implementation', 'frontend-review', 'frontend-ux')
ROOT = Path(__file__).resolve().parents[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true', help='Inspect paths without writing')
    arguments = parser.parse_args()
    canonical = ROOT / '.agents/skills'
    # Validate every source/destination before moving anything; never overwrite a directory.
    pending = []
    for name in NAMES:
        source = ROOT / '.opencode/skills' / name
        target = canonical / name
        if target.is_dir() and source.is_symlink() and source.resolve() == target.resolve():
            continue
        if source.is_symlink() or not (source / 'SKILL.md').is_file() or target.exists() or target.is_symlink():
            parser.error('Unexpected skill paths: ' + name)
        claude = ROOT / '.claude/skills' / name
        if claude.exists() or claude.is_symlink():
            if not claude.is_symlink() or claude.resolve() != source.resolve():
                parser.error('Unexpected Claude destination: ' + name)
        pending.append((name, source, target, claude))
    for name, source, target, claude in pending:
        print(name + ': .opencode/skills -> .agents/skills; relative CLI links')
        if arguments.check:
            continue
        source.rename(target)
        source.symlink_to(os.path.relpath(target, source.parent), target_is_directory=True)
        if claude.is_symlink():
            claude.unlink()
        claude.symlink_to(os.path.relpath(target, claude.parent), target_is_directory=True)
    for base in (ROOT / '.claude/skills', ROOT / '.opencode/skills'):
        for target in canonical.glob('openspec-*'):
            link = base / target.name
            if not link.exists() and not link.is_symlink() and not arguments.check:
                link.symlink_to(os.path.relpath(target, base), target_is_directory=True)
    print('Check only; no files moved.' if arguments.check else 'Skill move complete.')


if __name__ == '__main__':
    main()
