from __future__ import annotations

import os
import shutil
import subprocess
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable


def _split_csv(raw: str, default: list[str]) -> list[str]:
    values = [item.strip().lower() for item in raw.split(",") if item.strip()]
    return values or default


@dataclass(slots=True)
class RAGConfig:
    rag_cli_path: Path
    db_path: Path
    data_dir: Path
    upload_dir: Path
    vec0_path: Path | None
    timeout_seconds: int
    default_k: int
    default_max_tokens: int
    allowed_extensions: list[str]
    max_upload_size_mb: int

    @classmethod
    def from_env(cls) -> "RAGConfig":
        cli_default = "../rag/integration/rag-cli.exe" if os.name == "nt" else "../rag/integration/rag-cli"
        vec0 = os.getenv("RAG_VEC0_PATH", "").strip()
        return cls(
            rag_cli_path=Path(os.getenv("RAG_CLI_PATH", cli_default)).resolve(),
            db_path=Path(os.getenv("RAG_DB_PATH", "data/rag.db")),
            data_dir=Path(os.getenv("RAG_DATA_DIR", "data")),
            upload_dir=Path(os.getenv("UPLOAD_DIR", "integration/uploads")),
            vec0_path=Path(vec0) if vec0 else None,
            timeout_seconds=int(os.getenv("RAG_TIMEOUT_SECONDS", "90")),
            default_k=int(os.getenv("RAG_K", "3")),
            default_max_tokens=int(os.getenv("RAG_MAX_TOKENS", "3000")),
            allowed_extensions=_split_csv(os.getenv("ALLOWED_EXTENSIONS", ".pdf"), [".pdf"]),
            max_upload_size_mb=int(os.getenv("MAX_UPLOAD_SIZE_MB", "50")),
        )


class RAGService:
    def __init__(
        self,
        config: RAGConfig | None = None,
        user_id: str | None = None,
        project_id: str | None = None,
    ) -> None:
        self.config = config or RAGConfig.from_env()
        if user_id:
            base = Path("data/users") / str(user_id)
            self.config.db_path = base / "rag.db"
            self.config.data_dir = base / "documents"
            self.config.upload_dir = base / "uploads"

    def health_check(self) -> dict[str, object]:
        cli_exists = self.config.rag_cli_path.exists()
        db_exists = self.config.db_path.exists()
        data_dir_exists = self.config.data_dir.exists()
        result = {
            "ok": bool(cli_exists and data_dir_exists),
            "cli_exists": cli_exists,
            "db_exists": db_exists,
            "data_dir_exists": data_dir_exists,
            "rag_cli_path": str(self.config.rag_cli_path),
            "rag_db_path": str(self.config.db_path),
            "rag_data_dir": str(self.config.data_dir),
            "upload_dir": str(self.config.upload_dir),
        }
        if cli_exists:
            version_probe = self._run(["help"], check=False)
            result["cli_ready"] = version_probe.returncode == 0
            if version_probe.stderr.strip():
                result["stderr"] = version_probe.stderr.strip()
        return result

    def query(self, query: str, chip_family: str = "", k: int | None = None, max_tokens: int | None = None) -> dict[str, object]:
        args = [
            "query",
            "--db",
            str(self.config.db_path),
            "--query",
            query,
            "--k",
            str(k or self.config.default_k),
            "--max-tokens",
            str(max_tokens or self.config.default_max_tokens),
        ]
        if chip_family:
            args.extend(["--chip-family", chip_family])
        if self.config.vec0_path:
            args.extend(["--cgo-path", str(self.config.vec0_path)])

        completed = self._run(args)
        return {
            "context": self._extract_context(completed.stdout),
            "stdout": completed.stdout,
            "stderr": completed.stderr,
            "returncode": completed.returncode,
        }

    def ingest(self, data_dir: str | Path | None = None, db_path: str | Path | None = None) -> dict[str, object]:
        target_dir = Path(data_dir) if data_dir else self.config.data_dir
        target_db = Path(db_path) if db_path else self.config.db_path
        args = ["ingest", "--db", str(target_db), "--dir", str(target_dir)]
        completed = self._run(args)
        return {
            "stdout": completed.stdout,
            "stderr": completed.stderr,
            "returncode": completed.returncode,
        }

    def evaluate(self, queries_path: str | Path, k: int | None = None) -> dict[str, object]:
        args = [
            "eval",
            "--db",
            str(self.config.db_path),
            "--queries",
            str(queries_path),
            "--k",
            str(k or self.config.default_k),
        ]
        if self.config.vec0_path:
            args.extend(["--cgo-path", str(self.config.vec0_path)])
        completed = self._run(args)
        return {
            "stdout": completed.stdout,
            "stderr": completed.stderr,
            "returncode": completed.returncode,
        }

    def stage_documents(self, files: Iterable[Path]) -> list[str]:
        self.config.data_dir.mkdir(parents=True, exist_ok=True)
        copied: list[str] = []
        for file_path in files:
            suffix = file_path.suffix.lower()
            if suffix not in self.config.allowed_extensions:
                raise ValueError(f"Unsupported file type: {file_path.name}")
            destination = self.config.data_dir / file_path.name
            shutil.copy2(file_path, destination)
            copied.append(str(destination))
        return copied

    def _run(self, args: list[str], check: bool = True) -> subprocess.CompletedProcess[str]:
        import time
        self.config.db_path.parent.mkdir(parents=True, exist_ok=True)
        self.config.data_dir.mkdir(parents=True, exist_ok=True)
        self.config.upload_dir.mkdir(parents=True, exist_ok=True)
        command = [str(self.config.rag_cli_path), *args]
        
        max_retries = 3
        for attempt in range(max_retries):
            completed = subprocess.run(
                command,
                text=True,
                capture_output=True,
                encoding="utf-8",
                errors="replace",
                timeout=self.config.timeout_seconds,
                check=False,
            )
            
            message = completed.stderr.strip() or completed.stdout.strip() or "rag-cli failed"
            
            if completed.returncode == 0:
                return completed
                
            if "database is locked" in message and attempt < max_retries - 1:
                time.sleep(1.0 * (attempt + 1))  # Exponential-ish backoff
                continue
                
            if check:
                raise RuntimeError(message)
                
            return completed

    @staticmethod
    def _extract_context(stdout: str | None) -> str:
        if not stdout:
            return ""
        marker = "=== LLM-READY PROMPT CONTEXT WINDOW ==="
        if marker not in stdout:
            return stdout.strip()
        _, context_block = stdout.split(marker, 1)
        lines = [line.rstrip() for line in context_block.splitlines()]
        trimmed = [line for line in lines if line.strip("- ").strip()]
        return "\n".join(trimmed).strip()
