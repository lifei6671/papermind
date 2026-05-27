export type ApiBody<T> = {
  code: number;
  message: string;
  data: T;
};

export type PageData<T> = {
  items: T[];
  page: number;
  page_size: number;
  total: number;
};

type ApiClientOptions = {
  baseUrl: string;
  fetcher?: typeof fetch;
  getAccessToken?: () => string | null | undefined;
  onUnauthorized?: () => void;
};

type RequestOptions = {
  method?: string;
  body?: unknown;
  headers?: Record<string, string>;
};

export class ApiError extends Error {
  code: number;
  status: number;
  body: unknown;

  constructor(message: string, code: number, status: number, body: unknown) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.status = status;
    this.body = body;
  }
}

const apiErrorMessageByCode: Record<number, string> = {
  40001: "请求参数不正确",
  401: "登录状态已失效，请重新登录",
  403: "没有权限执行当前操作",
  50000: "服务暂时不可用，请稍后重试",
};

export type ApiClient = {
  get<T>(path: string, options?: RequestOptions): Promise<T>;
  post<T>(path: string, body?: unknown, options?: RequestOptions): Promise<T>;
  upload<T>(path: string, formData: FormData, options?: RequestOptions): Promise<T>;
  request<T>(path: string, options?: RequestOptions): Promise<T>;
};

export function createApiClient(options: ApiClientOptions): ApiClient {
  async function request<T>(path: string, requestOptions: RequestOptions = {}): Promise<T> {
    const headers = buildHeaders(options.getAccessToken?.(), requestOptions);
    const fetcher = options.fetcher ?? fetch;
    const response = await fetcher(buildUrl(options.baseUrl, path), {
      method: requestOptions.method ?? "GET",
      ...(requestOptions.body === undefined ? {} : { body: serializeBody(requestOptions.body, headers) }),
      headers,
    });

    const body = await parseBody<unknown>(response);

    if (response.status === 401) {
      options.onUnauthorized?.();
    }

    if (!response.ok) {
      const message = readMessage(body) ?? `HTTP ${response.status}`;
      throw new ApiError(message, readCode(body) ?? response.status, response.status, body);
    }

    const envelope = body as ApiBody<T>;
    if (envelope.code !== 0) {
      throw new ApiError(envelope.message, envelope.code, response.status, body);
    }

    return envelope.data;
  }

  return {
    get: (path, requestOptions) => request(path, { ...requestOptions, method: "GET" }),
    post: (path, body, requestOptions) => request(path, { ...requestOptions, body, method: "POST" }),
    upload: (path, formData, requestOptions) =>
      request(path, { ...requestOptions, body: formData, method: "POST" }),
    request,
  };
}

export function formatApiErrorMessage(error: unknown, fallback = "操作失败") {
  if (error instanceof ApiError) {
    // 后端有明确业务文案时优先展示文案；无文案时按错误码映射统一提示。
    return error.message || apiErrorMessageByCode[error.code] || fallback;
  }
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return fallback;
}

function buildUrl(baseUrl: string, path: string) {
  if (!baseUrl) {
    return path;
  }

  return `${baseUrl.replace(/\/$/, "")}/${path.replace(/^\//, "")}`;
}

function buildHeaders(accessToken: string | null | undefined, options: RequestOptions) {
  const headers: Record<string, string> = {
    Accept: "application/json",
    ...options.headers,
  };

  if (accessToken) {
    headers.Authorization = `Bearer ${accessToken}`;
  }

  return headers;
}

function serializeBody(body: unknown, headers: Record<string, string>) {
  if (body instanceof FormData) {
    return body;
  }

  headers["Content-Type"] = "application/json";
  return JSON.stringify(body);
}

async function parseBody<T>(response: Response): Promise<T> {
  const text = await response.text();
  if (!text) {
    return null as T;
  }

  return JSON.parse(text) as T;
}

function readMessage(body: unknown) {
  if (body && typeof body === "object" && "message" in body && typeof body.message === "string") {
    return body.message;
  }

  return null;
}

function readCode(body: unknown) {
  if (body && typeof body === "object" && "code" in body && typeof body.code === "number") {
    return body.code;
  }

  return null;
}
