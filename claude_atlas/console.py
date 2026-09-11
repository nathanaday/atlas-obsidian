"""Terminal prompts and step reporting for the CLI."""

from __future__ import annotations

import sys
from typing import TextIO

from .errors import AtlasError


class Console:
    def __init__(self, *, assume_yes: bool = False, stream: TextIO | None = None):
        self.assume_yes = assume_yes
        self.stream = stream or sys.stdout

    @property
    def interactive(self) -> bool:
        return sys.stdin.isatty()

    def say(self, text: str = "") -> None:
        print(text, file=self.stream)

    def step(self, label: str, detail: str = "", *, status: str = "ok") -> None:
        marks = {"ok": "✓", "skip": "·", "fail": "✗", "run": "→"}
        mark = marks[status]
        line = f"  {mark} {label}"
        if detail:
            line += f"  {detail}"
        print(line, file=self.stream)

    def confirm(self, prompt: str, *, default: bool = True) -> bool:
        if self.assume_yes:
            return True
        if not self.interactive:
            raise AtlasError(
                "stdin is not a terminal; pass --yes to run without prompts"
            )
        suffix = "[Y/n]" if default else "[y/N]"
        while True:
            answer = input(f"{prompt} {suffix} ").strip().lower()
            if not answer:
                return default
            if answer in ("y", "yes"):
                return True
            if answer in ("n", "no"):
                return False

    def ask(self, prompt: str, default: str) -> str:
        if self.assume_yes or not self.interactive:
            return default
        answer = input(f"{prompt} [{default}]: ").strip()
        return answer or default
