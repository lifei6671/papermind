import { UploadCloud } from "lucide-react";
import { useEffect, useId, useState } from "react";

type FileUploadFieldProps = {
  label: string;
  accept?: string[];
  maxSizeBytes?: number;
  onFileAccepted: (file: File) => void;
  onReject?: (message: string) => void;
  previewSrc?: string;
  selectedLabel?: string;
  uploadPrompt?: string;
  emptyPreviewText?: string;
};

export function FileUploadField({
  label,
  accept = [],
  maxSizeBytes,
  onFileAccepted,
  onReject,
  previewSrc,
  selectedLabel,
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
    const file = event.currentTarget.files?.[0];
    if (!file) {
      return;
    }

    acceptFile(file, () => {
      event.currentTarget.value = "";
    });
  }

  function handleDrop(event: React.DragEvent<HTMLElement>) {
    event.preventDefault();
    const file = event.dataTransfer.files?.[0];
    if (!file) {
      return;
    }

    acceptFile(file);
  }

  function acceptFile(file: File, resetInput?: () => void) {
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

    setSelectedName(file.name);
    const canPreviewImage = file.type.startsWith("image/") && typeof URL.createObjectURL === "function";
    setPreviewURL(canPreviewImage ? URL.createObjectURL(file) : "");
    onFileAccepted(file);
  }

  return (
    <div className="file-upload-field">
      <label className="file-upload-field__label" htmlFor={inputID}>
        {label}
      </label>
      <div className="file-upload-field__layout">
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
            onChange={handleFileChange}
            onDragOver={(event) => event.preventDefault()}
            onDrop={handleDrop}
            type="file"
          />
        </div>
        <div aria-label={`${label}预览`} className="file-upload-field__preview">
          {previewURL || previewSrc ? (
            <img alt={`${selectedName || selectedLabel || label} 预览`} src={previewURL || previewSrc} />
          ) : (
            <span>{emptyPreviewText}</span>
          )}
        </div>
      </div>
      {maxSizeBytes !== undefined && <small>最大 {formatBytes(maxSizeBytes)}</small>}
    </div>
  );
}

function formatBytes(bytes: number) {
  if (bytes < 1024) {
    return `${bytes} B`;
  }

  return `${(bytes / 1024).toFixed(1)} KB`;
}
