import { createApiClient } from "./client";
import type { ApiClient } from "./client";
import { readStoredAccessToken } from "./session-token";

export type UploadFileInput = {
  category: string;
  file: File;
};

export type UploadFileResult = {
  key: string;
  url: string;
  fileName: string;
  contentType: string;
  size: number;
};

export type UploadAPI = {
  uploadFile(input: UploadFileInput): Promise<UploadFileResult>;
};

type UploadAPIResponse = {
  key: string;
  url: string;
  file_name: string;
  content_type: string;
  size: number;
};

const defaultApiClient = createApiClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? "",
  getAccessToken: readStoredAccessToken,
});

export const uploadApi = createUploadAPI(defaultApiClient);

export function createUploadAPI(apiClient: ApiClient): UploadAPI {
  return {
    async uploadFile(input) {
      const file = await convertImageToWebP(input.file);
      const formData = new FormData();
      formData.append("category", input.category);
      formData.append("file", file);
      const data = await apiClient.upload<UploadAPIResponse>("/api/v1/uploads", formData);
      return mapUploadResponse(data);
    },
  };
}

async function convertImageToWebP(file: File) {
  if (!file.type.startsWith("image/") || file.type === "image/webp" || typeof createImageBitmap !== "function") {
    return file;
  }

  try {
    const bitmap = await createImageBitmap(file);
    const canvas = document.createElement("canvas");
    canvas.width = bitmap.width;
    canvas.height = bitmap.height;
    const context = canvas.getContext("2d");
    if (!context) {
      bitmap.close();
      return file;
    }
    context.drawImage(bitmap, 0, 0);
    const blob = await new Promise<Blob | null>((resolve) => {
      canvas.toBlob(resolve, "image/webp", 0.92);
    });
    bitmap.close();
    if (!blob) {
      return file;
    }
    return new File([blob], replaceFileExtension(file.name, "webp"), {
      lastModified: file.lastModified,
      type: "image/webp",
    });
  } catch {
    return file;
  }
}

function replaceFileExtension(fileName: string, extension: string) {
  const dotIndex = fileName.lastIndexOf(".");
  if (dotIndex <= 0) {
    return `${fileName}.${extension}`;
  }
  return `${fileName.slice(0, dotIndex)}.${extension}`;
}

function mapUploadResponse(response: UploadAPIResponse): UploadFileResult {
  return {
    key: response.key,
    url: response.url,
    fileName: response.file_name,
    contentType: response.content_type,
    size: response.size,
  };
}
