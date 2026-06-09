import { fireEvent, render, screen } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import { PlatformDrawer } from "./PlatformDrawer";

test("平台抽屉使用 Ant Design 原生遮罩并支持点击遮罩关闭", () => {
  const onClose = vi.fn();

  render(
    <PlatformDrawer ariaLabel="测试抽屉" onClose={onClose} open>
      <div>抽屉内容</div>
    </PlatformDrawer>,
  );

  const drawer = screen.getByRole("dialog", { name: "测试抽屉" });
  expect(drawer.closest(".ant-drawer")).toBeInTheDocument();
  expect(screen.queryByTestId("tenant-resource-drawer-layer")).not.toBeInTheDocument();

  const mask = document.querySelector(".ant-drawer-mask");
  expect(mask).toBeInTheDocument();
  fireEvent.click(mask as HTMLElement);

  expect(onClose).toHaveBeenCalledTimes(1);
});
