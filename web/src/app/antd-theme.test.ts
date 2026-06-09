import { expect, test } from "vitest";
import { paperMindAntTheme } from "./antd-theme";

test("AntD 主题对齐 PaperMind 现有后台视觉 token", () => {
  expect(paperMindAntTheme.token?.colorPrimary).toBe("#dc7556");
  expect(paperMindAntTheme.token?.borderRadius).toBe(8);
  expect(paperMindAntTheme.token?.fontSize).toBe(13);
  expect(paperMindAntTheme.components?.Button?.borderRadius).toBe(8);
});
