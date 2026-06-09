import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement } from "react";
import { MemoryRouter } from "react-router-dom";
import { expect, test, vi } from "vitest";
import type { ActorRole } from "../../api/grading";
import { FeedbackProvider } from "../../app/feedback";
import type { QuestionImportAPI } from "../../api/questions";
import type { SpaceManagementAPI } from "../../api/spaces";
import { QuestionImportPage } from "./QuestionImportPage";

function renderWithFeedback(page: ReactElement) {
  return render(
    <FeedbackProvider>
      <MemoryRouter>{page}</MemoryRouter>
    </FeedbackProvider>,
  );
}

function renderImportPage(api: QuestionImportAPI, actorRole?: ActorRole) {
  renderWithFeedback(<QuestionImportPage actorRole={actorRole} api={api} tenantID={10} spaceID={301} />);
}

function createSpaceAPI(): Pick<SpaceManagementAPI, "listSpaces"> {
  return {
    listSpaces: vi.fn().mockResolvedValue({
      items: [
        {
          id: 301,
          tenantID: 10,
          name: "高一一班",
          description: "高一一班空间",
          logoFileName: "class-a.png",
          members: [],
        },
        {
          id: 302,
          tenantID: 10,
          name: "高一二班",
          description: "高一二班空间",
          logoFileName: "class-b.png",
          members: [],
        },
      ],
    }),
  };
}

function renderTenantAdminImportPage(api: QuestionImportAPI, spaceApi = createSpaceAPI()) {
  renderWithFeedback(<QuestionImportPage actorRole="tenant_admin" api={api} spaceApi={spaceApi} tenantID={10} />);
  return spaceApi;
}

test("题目导入页可以上传文件并展示解析结果", async () => {
  const user = userEvent.setup();
  const api: QuestionImportAPI = {
    importQuestions: async (input) => {
      expect(input.tenantID).toBe(10);
      expect(input.file.name).toBe("questions.csv");
      return {
        successCount: 1,
        duplicateCount: 0,
        errors: [{ rowNumber: 3, reason: "choice question needs correct answer" }],
      };
    },
  };
  renderWithFeedback(<QuestionImportPage api={api} tenantID={10} />);

  expect(screen.getAllByRole("tab")).toHaveLength(1);
  expect(screen.getByRole("tab", { name: "题目导入" })).toHaveAttribute("aria-selected", "true");
  expect(screen.queryByRole("heading", { name: "题目导入" })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "导入题目" })).toHaveClass("tenant-create-button");
  expect(screen.getByLabelText("搜索导入记录")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "刷新导入记录" })).toBeInTheDocument();
  expect(screen.queryByLabelText("题目导入文件")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "导入题目" }));

  expect(screen.getByRole("dialog", { name: "导入题目弹窗" })).toBeInTheDocument();
  expect(screen.queryByLabelText("题目导入文件预览")).not.toBeInTheDocument();
  const templateLink = screen.getByRole("link", { name: "下载 CSV 模板" });
  expect(templateLink).toHaveAttribute("download", "question-import-template.csv");
  expect(templateLink).toHaveAttribute("href", expect.stringContaining("data:text/csv"));
  expect(screen.getByLabelText("题目导入文件")).toHaveAttribute("accept", "text/csv");

  await user.upload(screen.getByLabelText("题目导入文件"), new File(["title,type"], "questions.csv", { type: "text/csv" }));
  expect(screen.getByText("questions.csv")).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "确认导入" }));

  const alert = await screen.findByRole("alert");
  expect(alert).toHaveTextContent("questions.csv");
  expect(alert).toHaveTextContent("已导入 1 道题，1 行失败");
  expect(alert).toHaveTextContent("第 3 行");
  expect(alert.closest(".ant-message")).toBeInTheDocument();
  expect(document.querySelector(".tenant-admin-status")).not.toBeInTheDocument();
});

test("题目导入页可以选择导入到公共题库", async () => {
  const user = userEvent.setup();
  const api: QuestionImportAPI = {
    importQuestions: async (input) => {
      expect(input.tenantID).toBe(10);
      expect(input.spaceID).toBeNull();
      expect(input.file.name).toBe("questions.csv");
      return { successCount: 1, duplicateCount: 0, errors: [] };
    },
  };
  renderImportPage(api);

  await user.click(screen.getByRole("button", { name: "导入题目" }));
  await user.selectOptions(screen.getByLabelText("导入所属空间"), "public");
  await user.upload(screen.getByLabelText("题目导入文件"), new File(["title,type"], "questions.csv", { type: "text/csv" }));
  await user.click(screen.getByRole("button", { name: "确认导入" }));

  expect(await screen.findByRole("alert")).toHaveTextContent("已导入 1 道题");
});

test("租户管理员可以在题目导入页选择导入到具体空间", async () => {
  const user = userEvent.setup();
  const api: QuestionImportAPI = {
    importQuestions: async (input) => {
      expect(input.tenantID).toBe(10);
      expect(input.spaceID).toBe(302);
      expect(input.file.name).toBe("questions.csv");
      return { successCount: 1, duplicateCount: 0, errors: [] };
    },
  };
  const spaceApi = renderTenantAdminImportPage(api);

  await user.click(screen.getByRole("button", { name: "导入题目" }));

  expect(screen.getByLabelText("导入所属空间").closest(".question-import-scope-field")).not.toBeNull();
  await screen.findByRole("option", { name: "高一二班" });
  expect(spaceApi.listSpaces).toHaveBeenCalledWith(10);

  await user.selectOptions(screen.getByLabelText("导入所属空间"), "space:302");
  await user.upload(screen.getByLabelText("题目导入文件"), new File(["title,type"], "questions.csv", { type: "text/csv" }));
  await user.click(screen.getByRole("button", { name: "确认导入" }));

  expect(await screen.findByRole("alert")).toHaveTextContent("已导入 1 道题");
});

test("空间管理员在独立导入页看不到公共题库选项", async () => {
  const user = userEvent.setup();
  const api: QuestionImportAPI = {
    importQuestions: async () => ({ successCount: 1, duplicateCount: 0, errors: [] }),
  };
  renderImportPage(api, "space_admin");

  await user.click(screen.getByRole("button", { name: "导入题目" }));

  expect(screen.queryByRole("option", { name: "公共题库" })).not.toBeInTheDocument();
  expect(screen.getByRole("option", { name: "当前空间题库" })).toBeInTheDocument();
});

test("题目导入页可以搜索和刷新导入记录", async () => {
  const user = userEvent.setup();
  const api: QuestionImportAPI = {
    importQuestions: async () => ({ successCount: 1, duplicateCount: 0, errors: [] }),
  };
  renderWithFeedback(<QuestionImportPage api={api} />);

  expect(screen.queryByText("sample-questions.csv")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "导入题目" }));
  await user.upload(screen.getByLabelText("题目导入文件"), new File(["title,type"], "questions.csv", { type: "text/csv" }));
  await user.click(screen.getByRole("button", { name: "确认导入" }));

  expect(screen.getByText("questions.csv")).toBeInTheDocument();

  await user.type(screen.getByLabelText("搜索导入记录"), "不存在");
  await user.click(screen.getByRole("button", { name: "搜索" }));

  expect(screen.queryByText("questions.csv")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "刷新导入记录" }));

  expect(screen.getByRole("button", { name: "刷新导入记录" }).querySelector("svg")).toHaveClass(
    "tenant-refresh-icon--spinning",
  );
  expect(screen.getByText("questions.csv")).toBeInTheDocument();
});

test("取消导入后不会保留上一次选择的文件", async () => {
  const user = userEvent.setup();
  const api: QuestionImportAPI = {
    importQuestions: async () => {
      throw new Error("不应该提交已取消的文件");
    },
  };
  renderWithFeedback(<QuestionImportPage api={api} tenantID={10} />);

  await user.click(screen.getByRole("button", { name: "导入题目" }));
  await user.upload(screen.getByLabelText("题目导入文件"), new File(["title,type"], "questions.csv", { type: "text/csv" }));
  await user.click(screen.getByRole("button", { name: "取消" }));

  await user.click(screen.getByRole("button", { name: "导入题目" }));
  await user.click(screen.getByRole("button", { name: "确认导入" }));

  const alert = await screen.findByRole("alert");
  expect(alert).toHaveTextContent("请选择题目导入文件");
  expect(alert.closest(".ant-message")).toBeInTheDocument();
  expect(document.querySelector(".tenant-admin-status")).not.toBeInTheDocument();
});
