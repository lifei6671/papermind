import { afterEach, expect, test } from "vitest";
import { SESSION_STORAGE_KEY } from "../auth/session-context";
import { readStoredAccessToken } from "./session-token";

afterEach(() => {
  window.localStorage.clear();
});

test("从前端登录态读取 access token 供业务 API 自动注入", () => {
  window.localStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify({
    accessToken: "platform-access-token",
    refreshToken: "platform-refresh-token",
    user: {
      displayName: "admin",
      role: "platform_admin",
      userID: 1,
    },
  }));

  expect(readStoredAccessToken()).toBe("platform-access-token");
});

test("登录态缺失或损坏时不为业务 API 注入 token", () => {
  expect(readStoredAccessToken()).toBeNull();

  window.localStorage.setItem(SESSION_STORAGE_KEY, "{broken");

  expect(readStoredAccessToken()).toBeNull();
});
