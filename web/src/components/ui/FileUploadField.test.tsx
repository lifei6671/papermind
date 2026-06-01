import { fireEvent, render, screen, within } from "@testing-library/react";
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

test("文件上传组件支持非图片上传文案", () => {
  render(
    <FileUploadField
      emptyPreviewText="暂未选择文件"
      label="题目导入文件"
      onFileAccepted={vi.fn()}
      uploadPrompt="选择文件或拖动文件到此处"
    />,
  );

  expect(screen.getByText("选择文件或拖动文件到此处")).toBeInTheDocument();
  expect(within(screen.getByLabelText("题目导入文件预览")).getByText("暂未选择文件")).toBeInTheDocument();
  expect(screen.queryByText("选择图片或拖动图片到此处")).not.toBeInTheDocument();
  expect(screen.queryByText("暂未选择图片")).not.toBeInTheDocument();
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

test("文件上传组件提供拖拽上传区域和图片预览区域", () => {
  const createObjectURL = vi.fn().mockReturnValue("blob:logo-preview");
  const revokeObjectURL = vi.fn();
  vi.stubGlobal("URL", {
    ...URL,
    createObjectURL,
    revokeObjectURL,
  });
  const onFileAccepted = vi.fn();

  render(
    <FileUploadField
      accept={["image/png"]}
      label="租户 Logo"
      maxSizeBytes={1024}
      onFileAccepted={onFileAccepted}
    />,
  );

  expect(screen.getByLabelText("租户 Logo上传区域")).toBeInTheDocument();
  const preview = screen.getByLabelText("租户 Logo预览");
  expect(within(preview).getByText("暂未选择图片")).toBeInTheDocument();

  const logo = new File(["logo"], "tenant.png", { type: "image/png" });
  fireEvent.drop(screen.getByLabelText("租户 Logo上传区域"), {
    dataTransfer: { files: [logo] },
  });

  expect(onFileAccepted).toHaveBeenCalledWith(expect.objectContaining({ name: "tenant.png" }));
  expect(createObjectURL).toHaveBeenCalledWith(logo);
  expect(screen.getByRole("img", { name: "tenant.png 预览" })).toHaveAttribute("src", "blob:logo-preview");
  vi.unstubAllGlobals();
});

test("文件拖到真实文件输入框时也会生成图片预览", () => {
  const createObjectURL = vi.fn().mockReturnValue("blob:input-drop-preview");
  vi.stubGlobal("URL", {
    ...URL,
    createObjectURL,
    revokeObjectURL: vi.fn(),
  });
  const onFileAccepted = vi.fn();

  render(
    <FileUploadField
      accept={["image/png"]}
      label="租户 Logo"
      maxSizeBytes={1024}
      onFileAccepted={onFileAccepted}
    />,
  );

  const logo = new File(["logo"], "tenant-input.png", { type: "image/png" });
  fireEvent.drop(screen.getByLabelText("租户 Logo"), {
    dataTransfer: { files: [logo] },
  });

  expect(onFileAccepted).toHaveBeenCalledWith(expect.objectContaining({ name: "tenant-input.png" }));
  expect(screen.getByRole("img", { name: "tenant-input.png 预览" })).toHaveAttribute("src", "blob:input-drop-preview");
  vi.unstubAllGlobals();
});
