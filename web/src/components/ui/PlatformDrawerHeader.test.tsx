import { render, screen, within } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import { PlatformDrawerHeader } from "./PlatformDrawerHeader";

test("抽屉头部统一展示返回、分隔线和标题", () => {
  const onBack = vi.fn();

  render(
    <PlatformDrawerHeader
      actions={<button type="button">全屏</button>}
      backLabel="返回列表"
      onBack={onBack}
      title="编辑空间"
    />,
  );

  const header = screen.getByRole("banner", { name: "抽屉头部" });
  expect(header).toHaveClass("tenant-resource-drawer__head", "tenant-resource-drawer__head--unified");
  expect(header).toHaveTextContent("返回列表|编辑空间");

  const backButton = within(header).getByRole("button", { name: "返回列表" });
  expect(backButton).toHaveClass("tenant-resource-drawer__back");
  expect(within(header).getByText("|")).toHaveClass("tenant-resource-drawer__separator");
  expect(within(header).getByText("编辑空间")).toHaveClass("tenant-resource-drawer__space-name");
  expect(within(header).getByRole("button", { name: "全屏" })).toBeInTheDocument();
});
