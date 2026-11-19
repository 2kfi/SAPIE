# SAPIE — Developer Note

Project: Steam API Emulator (SAPIE)

## Python rule

ALWAYS use a Python virtual environment for any Python tooling in this repo.
NEVER use system Python or install packages system-wide.

    python -m venv .venv
    source .venv/bin/activate

Use only the activated interpreter's `python`/`pip`. `.venv/` is gitignored.
