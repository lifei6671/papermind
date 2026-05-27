import { describe, expect, test, vi } from "vitest";
import { createApiClient } from "./client";
import { createUploadAPI } from "./uploads";

describe("upload api", () => {
  test("文件上传会提交 multipart 表单并映射返回地址", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          key: "tenant-logos/20260527/logo.png",
          url: "/uploads/tenant-logos/20260527/logo.png",
          file_name: "logo.png",
          content_type: "image/png",
          size: 4,
        },
      })),
    );
    const api = createUploadAPI(createApiClient({ baseUrl: "", fetcher }));
    const file = new File(["logo"], "logo.png", { type: "image/png" });

    await expect(api.uploadFile({ category: "tenant-logos", file })).resolves.toEqual({
      key: "tenant-logos/20260527/logo.png",
      url: "/uploads/tenant-logos/20260527/logo.png",
      fileName: "logo.png",
      contentType: "image/png",
      size: 4,
    });

    const request = fetcher.mock.calls[0][1] as RequestInit;
    const body = request.body as FormData;
    expect(fetcher).toHaveBeenCalledWith("/api/v1/uploads", expect.objectContaining({ method: "POST" }));
    expect(body.get("category")).toBe("tenant-logos");
    expect(body.get("file")).toBe(file);
  });

  test("上传图片前会优先转换为 webp", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          key: "tenant-logos/20260527/logo.webp",
          url: "/uploads/tenant-logos/20260527/logo.webp",
          file_name: "logo.webp",
          content_type: "image/webp",
          size: 4,
        },
      })),
    );
    const bitmap = { close: vi.fn(), height: 12, width: 16 };
    vi.stubGlobal("createImageBitmap", vi.fn().mockResolvedValue(bitmap));
    const getContext = vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue({
      drawImage: vi.fn(),
    } as unknown as CanvasRenderingContext2D);
    const toBlob = vi.spyOn(HTMLCanvasElement.prototype, "toBlob").mockImplementation((callback, type) => {
      callback(new Blob(["webp"], { type: type ?? "image/webp" }));
    });
    const api = createUploadAPI(createApiClient({ baseUrl: "", fetcher }));
    const file = new File(["png"], "logo.png", { type: "image/png" });

    await api.uploadFile({ category: "tenant-logos", file });

    const request = fetcher.mock.calls[0][1] as RequestInit;
    const body = request.body as FormData;
    const uploadedFile = body.get("file") as File;
    expect(uploadedFile.name).toBe("logo.webp");
    expect(uploadedFile.type).toBe("image/webp");
    expect(bitmap.close).toHaveBeenCalled();

    getContext.mockRestore();
    toBlob.mockRestore();
    vi.unstubAllGlobals();
  });

  test("图片转 webp 失败时会上传原图", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          key: "tenant-logos/20260527/logo.png",
          url: "/uploads/tenant-logos/20260527/logo.png",
          file_name: "logo.png",
          content_type: "image/png",
          size: 3,
        },
      })),
    );
    vi.stubGlobal("createImageBitmap", vi.fn().mockRejectedValue(new Error("decode failed")));
    const api = createUploadAPI(createApiClient({ baseUrl: "", fetcher }));
    const file = new File(["png"], "logo.png", { type: "image/png" });

    await api.uploadFile({ category: "tenant-logos", file });

    const request = fetcher.mock.calls[0][1] as RequestInit;
    const body = request.body as FormData;
    expect(body.get("file")).toBe(file);
    vi.unstubAllGlobals();
  });
});
