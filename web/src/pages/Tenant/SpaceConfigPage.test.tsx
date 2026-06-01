import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";
import { SpaceConfigPage } from "./SpaceConfigPage";

test("空间配置页保存当前配置", async () => {
  const user = userEvent.setup();
  render(<SpaceConfigPage tenantID={10} />);

  expect(screen.getByRole("tab", { name: "空间配置" })).toHaveAttribute("aria-selected", "true");

  await user.clear(screen.getByLabelText("默认考试时长"));
  await user.type(screen.getByLabelText("默认考试时长"), "90");
  await user.click(screen.getByLabelText("允许学生查看练习解析"));
  await user.click(screen.getByRole("button", { name: "保存空间配置" }));

  expect(screen.getByRole("status")).toHaveTextContent("空间配置已保存");
  expect(screen.getByLabelText("默认考试时长")).toHaveValue(90);
  expect(screen.getByLabelText("允许学生查看练习解析")).not.toBeChecked();
});

test("缺少租户上下文时空间配置页展示提示", () => {
  render(<SpaceConfigPage />);

  expect(screen.getByRole("alert")).toHaveTextContent("当前账号没有租户上下文");
});
