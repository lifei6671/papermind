import { SESSION_STORAGE_KEY } from "../auth/session-context";
import type { AuthSession } from "../auth/session-context";

export function readStoredAccessToken() {
  const rawSession = window.localStorage.getItem(SESSION_STORAGE_KEY);
  if (!rawSession) {
    return null;
  }

  try {
    // 业务 API 默认 client 只读取登录态中的 accessToken，损坏的本地缓存不能阻断页面渲染。
    const session = JSON.parse(rawSession) as Partial<AuthSession>;
    return typeof session.accessToken === "string" && session.accessToken ? session.accessToken : null;
  } catch {
    return null;
  }
}
