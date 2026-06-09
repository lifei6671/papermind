import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";
import { useFeedback } from "./feedback-context";
import { FeedbackProvider } from "./feedback";

function FeedbackProbe() {
  const { clear, showError, showSuccess } = useFeedback();

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
      <button onClick={clear}>清空提示</button>
    </>
  );
}

afterEach(() => {
  vi.useRealTimers();
});

test("统一错误提示使用 AntD message 展示", async () => {
  const user = userEvent.setup();
  render(
    <FeedbackProvider>
      <FeedbackProbe />
    </FeedbackProvider>,
  );

  await user.click(screen.getByRole("button", { name: "显示错误" }));

  const message = await screen.findByText("租户码已失效");
  expect(message.closest(".ant-message")).toBeInTheDocument();
  expect(message.closest(".feedback-toast-stack")).not.toBeInTheDocument();
  expect(document.querySelector(".ant-alert")).not.toBeInTheDocument();
});

test("统一提示支持页面中上部叠加并可统一清空", async () => {
  render(
    <FeedbackProvider>
      <FeedbackProbe />
    </FeedbackProvider>,
  );

  fireEvent.click(screen.getByRole("button", { name: "显示多条提示" }));

  const errorMessage = await screen.findByText("租户码已失效");
  const successMessage = await screen.findByText("保存成功");
  expect(errorMessage.closest(".ant-message")).toBeInTheDocument();
  expect(successMessage.closest(".ant-message")).toBeInTheDocument();
  expect(document.querySelectorAll(".ant-message-notice")).toHaveLength(2);
  expect(document.querySelector(".feedback-toast-stack")).not.toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: "清空提示" }));
  await act(async () => {
    await Promise.resolve();
  });
  await waitFor(() => {
    expect(screen.queryByText("租户码已失效")).not.toBeInTheDocument();
    expect(screen.queryByText("保存成功")).not.toBeInTheDocument();
  });
});
