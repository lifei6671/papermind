import { UploadCloud } from "lucide-react";
import { useEffect, useId, useState } from "react";
import type { ReactNode } from "react";

type FileUploadFieldProps = {
  label: string;
  accept?: string[];
  maxSizeBytes?: number;
  onFileAccepted: (file: File) => void;
  onFilesAccepted?: (files: File[]) => void;
  onReject?: (message: string) => void;
  previewSrc?: string;
  selectedLabel?: string;
  showPreview?: boolean;
  multiple?: boolean;
  helperAction?: ReactNode;
  uploadPrompt?: string;
  emptyPreviewText?: string;
};

export function FileUploadField({
  label,
  accept = [],
  maxSizeBytes,
  onFileAccepted,
  onFilesAccepted,
  onReject,
  previewSrc,
  selectedLabel,
  showPreview = true,
  multiple = false,
  helperAction,
  uploadPrompt = "选择图片或拖动图片到此处",
  emptyPreviewText = "暂未选择图片",
}: FileUploadFieldProps) {
  const inputID = useId();
  const [selectedName, setSelectedName] = useState("");
  const [previewURL, setPreviewURL] = useState("");

  useEffect(() => {
    return () => {
      if (previewURL && typeof URL.revokeObjectURL === "function") {
        URL.revokeObjectURL(previewURL);
      }
    };
  }, [previewURL]);

  function handleFileChange(event: React.ChangeEvent<HTMLInputElement>) {
    const files = Array.from(event.currentTarget.files ?? []);
    if (files.length === 0) {
      return;
    }

    acceptFiles(files, () => {
      event.currentTarget.value = "";
    });
  }

  function handleDrop(event: React.DragEvent<HTMLElement>) {
    event.preventDefault();
    const files = Array.from(event.dataTransfer.files ?? []);
    if (files.length === 0) {
      return;
    }

    acceptFiles(files);
  }

  function acceptFiles(files: File[], resetInput?: () => void) {
    const acceptedFiles = multiple ? files : files.slice(0, 1);
    for (const file of acceptedFiles) {
      if (maxSizeBytes !== undefined && file.size > maxSizeBytes) {
        resetInput?.();
        setSelectedName("");
        setPreviewURL("");
        onReject?.(`文件不能超过 ${formatBytes(maxSizeBytes)}`);
        return;
      }

      if (accept.length > 0 && !accept.includes(file.type)) {
        resetInput?.();
        setSelectedName("");
        setPreviewURL("");
        onReject?.("文件类型不符合要求");
        return;
      }
    }

    const firstFile = acceptedFiles[0];
    setSelectedName(acceptedFiles.length === 1 ? firstFile.name : `已选择 ${acceptedFiles.length} 个文件`);
    const canPreviewImage = showPreview && firstFile.type.startsWith("image/") && typeof URL.createObjectURL === "function";
    setPreviewURL(canPreviewImage ? URL.createObjectURL(firstFile) : "");
    onFileAccepted(firstFile);
    onFilesAccepted?.(acceptedFiles);
  }

  const layoutClassName = [
    "file-upload-field__layout",
    showPreview ? "" : "file-upload-field__layout--single",
  ].filter(Boolean).join(" ");

  return (
    <div className="file-upload-field">
      <label className="file-upload-field__label" htmlFor={inputID}>
        {label}
      </label>
      <div className={layoutClassName}>
        <div
          aria-label={`${label}上传区域`}
          className="file-upload-field__control"
          onDragOver={(event) => event.preventDefault()}
          onDrop={handleDrop}
          role="button"
          tabIndex={0}
        >
          <UploadCloud aria-hidden="true" size={18} />
          <span>{selectedName || selectedLabel || uploadPrompt}</span>
          <input
            accept={accept.join(",")}
            id={inputID}
            multiple={multiple}
            onChange={handleFileChange}
            onDragOver={(event) => event.preventDefault()}
            onDrop={handleDrop}
            type="file"
          />
        </div>
        {showPreview && (
          <div aria-label={`${label}预览`} className="file-upload-field__preview">
            {previewURL || previewSrc ? (
              <img alt={`${selectedName || selectedLabel || label} 预览`} src={previewURL || previewSrc} />
            ) : (
              <span>{emptyPreviewText}</span>
            )}
          </div>
        )}
      </div>
      {(maxSizeBytes !== undefined || helperAction) && (
        <div className="file-upload-field__meta">
          {maxSizeBytes !== undefined && <small>最大 {formatBytes(maxSizeBytes)}</small>}
          {helperAction}
        </div>
      )}
    </div>
  );
}

function formatBytes(bytes: number) {
  if (bytes < 1024) {
    return `${bytes} B`;
  }

  return `${(bytes / 1024).toFixed(1)} KB`;
}
