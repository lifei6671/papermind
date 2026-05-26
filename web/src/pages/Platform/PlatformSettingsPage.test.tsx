import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";
import { PlatformSettingsPage } from "./PlatformSettingsPage";

test("平台配置页展示注册、安全和跨域配置", () => {
  render(<PlatformSettingsPage />);

  expect(screen.getAllByRole("tab")).toHaveLength(1);
  expect(screen.getByRole("tab", { name: "平台配置" })).toHaveAttribute("aria-selected", "true");
  expect(screen.queryByRole("heading", { name: "平台配置" })).not.toBeInTheDocument();
  expect(screen.getByLabelText("新租户默认允许自注册")).toBeChecked();
  expect(screen.getByLabelText("密码最小长度")).toHaveValue(8);
  expect(screen.getByLabelText("CORS 允许来源")).toHaveValue("http://localhost:5173");
});

test("平台配置保存后展示最新配置摘要", async () => {
  const user = userEvent.setup();
  render(<PlatformSettingsPage />);

  await user.click(screen.getByLabelText("新租户默认允许自注册"));
  await user.clear(screen.getByLabelText("密码最小长度"));
  await user.type(screen.getByLabelText("密码最小长度"), "10");
  await user.clear(screen.getByLabelText("CORS 允许来源"));
  await user.type(screen.getByLabelText("CORS 允许来源"), "https://exam.example.com");
  await user.click(screen.getByRole("button", { name: "保存平台配置" }));

  expect(screen.getByRole("status")).toHaveTextContent("默认关闭自注册");
  expect(screen.getByRole("status")).toHaveTextContent("密码至少 10 位");
  expect(screen.getByRole("status")).toHaveTextContent("https://exam.example.com");
});
