import { UploadCloud } from "lucide-react";
import { useId, useState } from "react";

type FileUploadFieldProps = {
  label: string;
  accept?: string[];
  maxSizeBytes?: number;
  onFileAccepted: (file: File) => void;
  onReject?: (message: string) => void;
};

export function FileUploadField({
  label,
  accept = [],
  maxSizeBytes,
  onFileAccepted,
  onReject,
}: FileUploadFieldProps) {
  const inputID = useId();
  const [selectedName, setSelectedName] = useState("");

  function handleFileChange(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.currentTarget.files?.[0];
    if (!file) {
      return;
    }

    if (maxSizeBytes !== undefined && file.size > maxSizeBytes) {
      event.currentTarget.value = "";
      setSelectedName("");
      onReject?.(`文件不能超过 ${formatBytes(maxSizeBytes)}`);
      return;
    }

    if (accept.length > 0 && !accept.includes(file.type)) {
      event.currentTarget.value = "";
      setSelectedName("");
      onReject?.("文件类型不符合要求");
      return;
    }

    setSelectedName(file.name);
    onFileAccepted(file);
  }

  return (
    <div className="file-upload-field">
      <label className="file-upload-field__label" htmlFor={inputID}>
        {label}
      </label>
      <div className="file-upload-field__control">
        <UploadCloud aria-hidden="true" size={18} />
        <span>{selectedName || "选择文件"}</span>
        <input accept={accept.join(",")} id={inputID} onChange={handleFileChange} type="file" />
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
