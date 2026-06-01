import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";
import { useFeedback } from "./feedback-context";
import { FeedbackProvider } from "./feedback";

function FeedbackProbe() {
  const { showError, showSuccess } = useFeedback();

  return (
    <>
      <button onClick={() => showError("租户码已失效")}>显示错误</button>
      <button
        onClick={() => {
          showError("租户码已失效");
          showSuccess("保存成功");
        }}
      >
        显示多条提示
      </button>
    </>
  );
}

afterEach(() => {
  vi.useRealTimers();
});

test("统一错误提示以 alert 展示并支持关闭", async () => {
  const user = userEvent.setup();
  render(
    <FeedbackProvider>
      <FeedbackProbe />
    </FeedbackProvider>,
  );

  await user.click(screen.getByRole("button", { name: "显示错误" }));

  expect(screen.getByRole("alert")).toHaveTextContent("租户码已失效");

  await user.click(screen.getByRole("button", { name: "关闭提示" }));

  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
});

test("统一提示支持页面中上部叠加并在 5 秒后自动消失", () => {
  vi.useFakeTimers();
  render(
    <FeedbackProvider>
      <FeedbackProbe />
    </FeedbackProvider>,
  );

  fireEvent.click(screen.getByRole("button", { name: "显示多条提示" }));

  const alerts = screen.getAllByRole("alert");
  expect(alerts).toHaveLength(2);
  expect(alerts[0]).toHaveTextContent("租户码已失效");
  expect(alerts[1]).toHaveTextContent("保存成功");
  expect(alerts[0].parentElement).toHaveClass("feedback-toast-stack");

  act(() => {
    vi.advanceTimersByTime(4999);
  });
  expect(screen.getAllByRole("alert")).toHaveLength(2);

  act(() => {
    vi.advanceTimersByTime(1);
  });
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
});
