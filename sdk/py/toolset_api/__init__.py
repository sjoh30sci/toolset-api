"""Hand-written Python client for the Toolset API.

For a fully-typed client generated from the live OpenAPI spec, run
``make sdk-generate`` (or the CI ``sdk-generate`` job) which emits an
openapi-generator client into ``./generated``. This module is a lightweight,
dependency-minimal wrapper mirroring the core endpoints.
"""

from __future__ import annotations

from typing import Any, Dict, List, Optional

import requests

__all__ = ["ToolsetAPI"]
__version__ = "0.1.0"


class ToolsetAPI:
    """Client for the Toolset API gateway."""

    def __init__(
        self,
        base_url: str = "http://localhost:8080",
        token: Optional[str] = None,
        timeout: float = 30.0,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self.session = requests.Session()
        if token:
            self.session.headers["Authorization"] = f"Bearer {token}"

    # --- internal helpers -------------------------------------------------
    def _post(self, path: str, payload: Dict[str, Any]) -> Any:
        resp = self.session.post(
            f"{self.base_url}{path}", json=payload, timeout=self.timeout
        )
        resp.raise_for_status()
        return resp.json()

    def _get(self, path: str) -> Any:
        resp = self.session.get(f"{self.base_url}{path}", timeout=self.timeout)
        resp.raise_for_status()
        return resp.json()

    def _delete(self, path: str) -> Any:
        resp = self.session.delete(f"{self.base_url}{path}", timeout=self.timeout)
        resp.raise_for_status()
        return resp.json()

    # --- system -----------------------------------------------------------
    def health(self) -> Dict[str, Any]:
        return self._get("/health")

    # --- search -----------------------------------------------------------
    def search(
        self,
        query: str,
        engines: Optional[List[str]] = None,
        page: Optional[int] = None,
        lang: Optional[str] = None,
    ) -> Dict[str, Any]:
        payload: Dict[str, Any] = {"query": query}
        if engines is not None:
            payload["engines"] = engines
        if page is not None:
            payload["page"] = page
        if lang is not None:
            payload["lang"] = lang
        return self._post("/search", payload)

    # --- exec -------------------------------------------------------------
    def exec(
        self,
        code: str,
        language: str,
        stdin: Optional[str] = None,
        timeout: Optional[int] = None,
    ) -> Dict[str, Any]:
        payload: Dict[str, Any] = {"code": code, "language": language}
        if stdin is not None:
            payload["stdin"] = stdin
        if timeout is not None:
            payload["timeout"] = timeout
        return self._post("/exec", payload)

    def exec_async(self, code: str, language: str, **kwargs: Any) -> Dict[str, Any]:
        payload: Dict[str, Any] = {"code": code, "language": language, **kwargs}
        return self._post("/exec/async", payload)

    def exec_status(self, job_id: str) -> Dict[str, Any]:
        return self._get(f"/exec/{job_id}")

    def exec_cancel(self, job_id: str) -> Dict[str, Any]:
        return self._delete(f"/exec/{job_id}")

    # --- files ------------------------------------------------------------
    def file_read(self, path: str) -> Dict[str, Any]:
        return self._post("/files/read", {"path": path})

    def file_write(self, path: str, content: str) -> Dict[str, Any]:
        return self._post("/files/write", {"path": path, "content": content})

    def file_list(self, path: str) -> Dict[str, Any]:
        return self._post("/files/list", {"path": path})

    def file_delete(self, path: str) -> Dict[str, Any]:
        return self._post("/files/delete", {"path": path})

    def file_move(self, path: str, destination: str) -> Dict[str, Any]:
        return self._post("/files/move", {"path": path, "destination": destination})

    # --- browser ----------------------------------------------------------
    def browser_create_session(self, browser_type: str = "chromium") -> Dict[str, Any]:
        return self._post("/browser/session", {"browserType": browser_type})

    def browser_get_session(self, session_id: str) -> Dict[str, Any]:
        return self._get(f"/browser/session/{session_id}")

    def browser_delete_session(self, session_id: str) -> Dict[str, Any]:
        return self._delete(f"/browser/session/{session_id}")

    def browser_action(
        self, session_id: str, action: Dict[str, Any]
    ) -> Dict[str, Any]:
        return self._post(
            "/browser/action", {"session_id": session_id, "action": action}
        )
