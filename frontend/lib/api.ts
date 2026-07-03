// Thin typed client for the Go API. Reads the JWT from localStorage and attaches
// it as a bearer token. All calls go through `request` so error handling and
// auth are consistent.

export const API_URL =
  process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

const TOKEN_KEY = "acp_token";

export function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return window.localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string) {
  window.localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken() {
  window.localStorage.removeItem(TOKEN_KEY);
}

export class ApiError extends Error {
  status: number;
  code: string;
  constructor(status: number, code: string, message: string) {
    super(message || code);
    this.status = status;
    this.code = code;
  }
}

async function request<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const headers = new Headers(options.headers);
  const token = getToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  if (!(options.body instanceof FormData) && options.body) {
    headers.set("Content-Type", "application/json");
  }

  const res = await fetch(`${API_URL}${path}`, { ...options, headers });
  const text = await res.text();
  const data = text ? JSON.parse(text) : null;

  if (!res.ok) {
    const code = data?.error || "error";
    const message = data?.message || res.statusText;
    if (res.status === 401) clearToken();
    throw new ApiError(res.status, code, message);
  }
  return data as T;
}

// ---- Types ----------------------------------------------------------------

export interface User { id: string; email: string; created_at: string; }
export interface AuthResult { token: string; user: User; }
export interface Role { id: string; name: string; required_skills: string[]; sample_jd: string; }
export interface Resume {
  id: string; filename: string; status: string;
  parsed?: any; created_at: string;
}
export interface RoadmapItem {
  order: number; topic: string; why: string;
  resources: string[]; milestone: string; status?: string;
}
export interface Analysis {
  id: string; role_id: string; role_name: string; gap_score: number;
  matched_skills: string[]; missing_skills: string[]; summary: string;
  roadmap: RoadmapItem[]; created_at: string;
}
export interface Question {
  id: string; role_id: string; topic: string; prompt: string;
  model_answer?: string; practiced: boolean;
}
export interface ModelAnswer { question: string; model_answer: string; key_points: string[]; }
export interface Score {
  question: string; answer: string;
  clarity: number; correctness: number; depth: number; feedback: string;
}
export interface Turn { role: string; content: string; }
export interface Session {
  id: string; role_id: string; role_name: string; status: string;
  transcript: Turn[]; scores: Score[]; created_at: string;
}
export interface TurnResult {
  session: Session; next_question?: string; last_score?: Score; done: boolean;
}
export interface Tailored {
  id: string; resume_id: string; job_description: string; content: any; created_at: string;
}

// ---- Endpoints ------------------------------------------------------------

export const api = {
  register: (email: string, password: string) =>
    request<AuthResult>("/api/auth/register", { method: "POST", body: JSON.stringify({ email, password }) }),
  login: (email: string, password: string) =>
    request<AuthResult>("/api/auth/login", { method: "POST", body: JSON.stringify({ email, password }) }),
  me: () => request<User>("/api/me"),

  roles: () => request<{ roles: Role[] }>("/api/roles"),

  uploadResume: (file: File) => {
    const form = new FormData();
    form.append("file", file);
    return request<Resume>("/api/resumes", { method: "POST", body: form });
  },
  listResumes: () => request<{ resumes: Resume[] }>("/api/resumes"),
  getResume: (id: string) => request<Resume>(`/api/resumes/${id}`),
  tailorResume: (id: string, jobDescription: string) =>
    request<Tailored>(`/api/resumes/${id}/tailor`, { method: "POST", body: JSON.stringify({ job_description: jobDescription }) }),

  analyze: (resumeId: string, roleId: string) =>
    request<Analysis>("/api/analyses", { method: "POST", body: JSON.stringify({ resume_id: resumeId, role_id: roleId }) }),
  listAnalyses: () => request<{ analyses: Analysis[] }>("/api/analyses"),
  getAnalysis: (id: string) => request<Analysis>(`/api/analyses/${id}`),
  setProgress: (id: string, itemOrder: number, status: string) =>
    request<{ ok: boolean }>(`/api/analyses/${id}/progress`, { method: "PATCH", body: JSON.stringify({ item_order: itemOrder, status }) }),

  listQuestions: (roleId: string, topic?: string) =>
    request<{ questions: Question[] }>(`/api/questions?role_id=${roleId}${topic ? `&topic=${encodeURIComponent(topic)}` : ""}`),
  searchQuestions: (q: string, roleId?: string) =>
    request<{ questions: Question[] }>(`/api/questions/search?q=${encodeURIComponent(q)}${roleId ? `&role_id=${roleId}` : ""}`),
  modelAnswer: (id: string, roleName: string) =>
    request<ModelAnswer>(`/api/questions/${id}/model-answer?role_name=${encodeURIComponent(roleName)}`),
  markPracticed: (id: string, practiced: boolean) =>
    request<{ ok: boolean }>(`/api/questions/${id}/practiced`, { method: "POST", body: JSON.stringify({ practiced }) }),

  startInterview: (roleId: string) =>
    request<TurnResult>("/api/interviews", { method: "POST", body: JSON.stringify({ role_id: roleId }) }),
  answerInterview: (id: string, answer: string) =>
    request<TurnResult>(`/api/interviews/${id}/answer`, { method: "POST", body: JSON.stringify({ answer }) }),
  getInterview: (id: string) => request<Session>(`/api/interviews/${id}`),
  listInterviews: () => request<{ sessions: Session[] }>("/api/interviews"),
};
