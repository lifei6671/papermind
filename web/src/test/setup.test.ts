import { expect, test } from "vitest";

test("测试环境提供标准 localStorage 方法", () => {
  window.localStorage.setItem("papermind.local-storage-probe", "ok");

  expect(window.localStorage.getItem("papermind.local-storage-probe")).toBe("ok");

  window.localStorage.clear();

  expect(window.localStorage.getItem("papermind.local-storage-probe")).toBeNull();
});
