// Thin, hand-written TypeScript client for the Toolset API.
//
// For a fully-typed client generated from the live OpenAPI spec, run
// `make sdk-generate` (or the CI `sdk-generate` job) which emits an
// openapi-generator client into ./generated. This hand-written wrapper is a
// convenient, dependency-light entry point that mirrors the core endpoints.
import axios, { AxiosInstance } from "axios";

export interface ToolsetOptions {
  /** Base URL of the gateway, e.g. http://localhost:8080 */
  baseURL?: string;
  /** Optional bearer token (required when the gateway runs in auth.mode=token). */
  token?: string;
  /** Request timeout in milliseconds. */
  timeoutMs?: number;
}

export interface SearchRequest {
  query: string;
  engines?: string[];
  page?: number;
  lang?: string;
}

export interface SearchResult {
  title: string;
  url: string;
  snippet: string;
  engine: string;
}

export interface SearchResponse {
  query: string;
  page: number;
  count: number;
  results: SearchResult[];
}

export interface ExecRequest {
  code: string;
  language: string;
  stdin?: string;
  timeout?: number;
}

export interface ExecResult {
  stdout: string;
  stderr: string;
  exit_code: number;
  duration: number;
}

export interface FileRequest {
  path: string;
  content?: string;
  destination?: string;
}

export class ToolsetAPI {
  private http: AxiosInstance;

  constructor(opts: ToolsetOptions = {}) {
    this.http = axios.create({
      baseURL: opts.baseURL ?? "http://localhost:8080",
      timeout: opts.timeoutMs ?? 30_000,
      headers: opts.token ? { Authorization: `Bearer ${opts.token}` } : {},
    });
  }

  async health(): Promise<Record<string, unknown>> {
    const { data } = await this.http.get("/health");
    return data;
  }

  async search(req: SearchRequest): Promise<SearchResponse> {
    const { data } = await this.http.post<SearchResponse>("/search", req);
    return data;
  }

  async exec(req: ExecRequest): Promise<ExecResult> {
    const { data } = await this.http.post<ExecResult>("/exec", req);
    return data;
  }

  async execAsync(req: ExecRequest): Promise<{ job_id: string; status: string }> {
    const { data } = await this.http.post("/exec/async", req);
    return data;
  }

  async execStatus(id: string): Promise<Record<string, unknown>> {
    const { data } = await this.http.get(`/exec/${id}`);
    return data;
  }

  files = {
    read: async (req: FileRequest) => (await this.http.post("/files/read", req)).data,
    write: async (req: FileRequest) => (await this.http.post("/files/write", req)).data,
    list: async (req: FileRequest) => (await this.http.post("/files/list", req)).data,
    delete: async (req: FileRequest) => (await this.http.post("/files/delete", req)).data,
    move: async (req: FileRequest) => (await this.http.post("/files/move", req)).data,
  };

  browser = {
    createSession: async (browserType = "chromium") =>
      (await this.http.post("/browser/session", { browserType })).data,
    getSession: async (id: string) => (await this.http.get(`/browser/session/${id}`)).data,
    deleteSession: async (id: string) => (await this.http.delete(`/browser/session/${id}`)).data,
    action: async (sessionId: string, action: Record<string, unknown>) =>
      (await this.http.post("/browser/action", { session_id: sessionId, action })).data,
  };
}

export default ToolsetAPI;
