import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import { FileUploadField } from "./FileUploadField";

test("文件上传组件接收符合类型和大小的文件", async () => {
  const user = userEvent.setup();
  const onFileAccepted = vi.fn();

  render(
    <FileUploadField
      accept={["text/csv"]}
      label="导入题目文件"
      maxSizeBytes={1024}
      onFileAccepted={onFileAccepted}
    />,
  );

  await user.upload(screen.getByLabelText("导入题目文件"), new File(["a,b"], "questions.csv", { type: "text/csv" }));

  expect(onFileAccepted).toHaveBeenCalledWith(expect.objectContaining({ name: "questions.csv" }));
  expect(screen.getByText("questions.csv")).toBeInTheDocument();
});

test("文件上传组件拒绝超过大小限制的文件", async () => {
  const user = userEvent.setup();
  const onReject = vi.fn();

  render(
    <FileUploadField
      label="上传头像"
      maxSizeBytes={4}
      onFileAccepted={vi.fn()}
      onReject={onReject}
    />,
  );

  await user.upload(screen.getByLabelText("上传头像"), new File(["12345"], "avatar.png", { type: "image/png" }));

  expect(onReject).toHaveBeenCalledWith("文件不能超过 4 B");
});
