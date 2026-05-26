import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";
import { useFeedback } from "./feedback-context";
import { FeedbackProvider } from "./feedback";

function FeedbackProbe() {
  const { showError } = useFeedback();

  return <button onClick={() => showError("租户码已失效")}>显示错误</button>;
}

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
