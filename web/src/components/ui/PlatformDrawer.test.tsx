import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useState } from "react";
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

test("平台抽屉在关闭动画结束后通知调用方", async () => {
  const afterOpenChange = vi.fn();

  function DrawerHarness() {
    const [open, setOpen] = useState(true);

    return (
      <PlatformDrawer
        afterOpenChange={afterOpenChange}
        ariaLabel="关闭动画抽屉"
        onClose={() => setOpen(false)}
        open={open}
      >
        <div>抽屉内容</div>
      </PlatformDrawer>
    );
  }

  render(<DrawerHarness />);

  fireEvent.click(document.querySelector(".ant-drawer-mask") as HTMLElement);

  await waitFor(() => expect(afterOpenChange).toHaveBeenCalledWith(false));
});

test("平台抽屉根节点锁定视口溢出避免动画期间出现页面滚动条", () => {
  const css = readFileSync(join(process.cwd(), "src/styles/global.css"), "utf8");
  const rootRule = css.match(/\.platform-ant-drawer-root\.ant-drawer\s*\{[\s\S]*?\}/)?.[0] ?? "";

  expect(rootRule).toContain("position: fixed !important");
  expect(rootRule).toContain("inset: 0 !important");
  expect(rootRule).toContain("max-width: 100vw");
  expect(rootRule).toContain("max-height: 100vh");
  expect(rootRule).toContain("overflow: hidden");
});
