import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { expect, test, vi } from "vitest";
import { ApiError } from "../../api/client";
import { FeedbackProvider } from "../../app/feedback";
import type { QuestionAPI } from "../../api/questions";
import type { SpaceManagementAPI } from "../../api/spaces";
import { QuestionBankPage } from "./QuestionBankPage";

const longQuestionTitle = "【资料】根据下列文字资料回答：2005年10月份，我国煤炭出口605万吨，同比增长25.9%，煤炭进口453万吨，同比下降28.7%。";

function renderWithFeedback(page: ReactElement, initialEntry = "/questions") {
  return render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={[initialEntry]}>{page}</MemoryRouter>
    </FeedbackProvider>,
  );
}

function LocationProbe() {
  const location = useLocation();

  return <output aria-label="当前路由">{`${location.pathname}${location.search}`}</output>;
}

function renderQuestionBankRoutes(api: ReturnType<typeof createQuestionAPI>) {
  return render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/questions?space_id=301"]}>
        <Routes>
          <Route
            path="/questions"
            element={(
              <>
                <QuestionBankPage api={api} tenantID={10} spaceID={301} />
                <LocationProbe />
              </>
            )}
          />
          <Route path="/questions/new" element={<LocationProbe />} />
          <Route path="/questions/:questionID/edit" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );
}

function renderQuestionBankRoutesWithRole(api: ReturnType<typeof createQuestionAPI>, actorRole: "teacher" | "space_admin" | "tenant_admin") {
  return render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/questions?space_id=301"]}>
        <Routes>
          <Route
            path="/questions"
            element={(
              <>
                <QuestionBankPage api={api} actorRole={actorRole} tenantID={10} spaceID={301} />
                <LocationProbe />
              </>
            )}
          />
          <Route path="/questions/new" element={<LocationProbe />} />
          <Route path="/questions/:questionID/edit" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });

  return { promise, resolve };
}

function createQuestionAPI() {
  return {
    listQuestions: vi.fn().mockResolvedValue({
      page: 1,
      pageSize: 20,
      total: 43,
      items: [
        {
          id: 100,
          tenantID: 10,
          type: "single",
          title: longQuestionTitle,
          stem: "下列选项最能概括文章中心的是哪一项？",
          options: ["把握中心句", "复述细节"],
          analysis: "定位中心句并排除以偏概全选项。",
          difficulty: "medium",
          tag: "阅读理解",
          tags: ["阅读理解"],
          scoreDefault: "4",
          authorName: "teacher01",
          authorRole: "teacher",
          createdAt: 1700000000000,
          status: "ready",
        },
        {
          id: 101,
          tenantID: 10,
          type: "short_text",
          title: "已禁用的阅读分析题",
          stem: "请分析文本结构。",
          options: [],
          analysis: "围绕结构层次作答。",
          difficulty: "hard",
          tag: "阅读理解",
          tags: ["阅读理解"],
          scoreDefault: "8",
          authorName: "admin01",
          authorRole: "space_admin",
          createdAt: 1700000060000,
          status: "disabled",
        },
        {
          id: 102,
          tenantID: 10,
          type: "fill_blank",
          title: "草稿状态题目",
          stem: "草稿状态题目",
          options: [],
          analysis: "草稿题需要手动启用。",
          difficulty: "easy",
          tag: "草稿",
          tags: ["草稿"],
          scoreDefault: "2",
          authorName: "teacher02",
          authorRole: "tenant_admin",
          createdAt: 1700000120000,
          status: "draft",
        },
      ],
    }),
    createQuestion: vi.fn().mockResolvedValue({
      id: 101,
      tenantID: 10,
      type: "single",
      title: "下列函数在 R 上单调递增的是哪一项？",
      stem: "下列函数在 R 上单调递增的是哪一项？",
      options: ["y = x", "y = -x"],
      analysis: "一次函数斜率为正时单调递增。",
      difficulty: "hard",
      tag: "函数",
      tags: ["阅读理解", "函数"],
      scoreDefault: "6",
      authorName: "teacher01",
      authorRole: "teacher",
      createdAt: 1700000000000,
      status: "ready",
    }),
    getQuestion: vi.fn(),
    updateQuestion: vi.fn(),
    deleteQuestion: vi.fn().mockResolvedValue(undefined),
    disableQuestion: vi.fn().mockResolvedValue({
      id: 100,
      tenantID: 10,
      type: "single",
      title: longQuestionTitle,
      stem: "下列选项最能概括文章中心的是哪一项？",
      options: ["把握中心句", "复述细节"],
      analysis: "定位中心句并排除以偏概全选项。",
      difficulty: "medium",
      tag: "阅读理解",
      tags: ["阅读理解"],
      scoreDefault: "4",
      authorName: "teacher01",
      authorRole: "teacher",
      createdAt: 1700000000000,
      status: "disabled",
    }),
    enableQuestion: vi.fn(),
    importQuestions: vi.fn().mockResolvedValue({
      successCount: 2,
      duplicateCount: 0,
      errors: [],
    }),
    startQuestionImportJob: vi.fn().mockResolvedValue({ jobID: "job-1" }),
    subscribeQuestionImportJob: vi.fn((_input, onEvent) => {
      onEvent({
        jobID: "job-1",
        status: "completed",
        fileName: "questions.csv",
        totalRows: 2,
        processedRows: 2,
        successCount: 2,
        errorCount: 0,
        duplicateCount: 0,
        errors: [],
      });
      return () => undefined;
    }),
  };
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

test("题库页展示题目列表并隐藏本地标签管理入口", async () => {
  const api = createQuestionAPI();
  renderWithFeedback(<QuestionBankPage api={api} tenantID={10} />);

  expect(screen.getAllByRole("tab")).toHaveLength(1);
  expect(screen.getByRole("tab", { name: "题库" })).toHaveAttribute("aria-selected", "true");
  expect(screen.queryByRole("heading", { name: "题库" })).not.toBeInTheDocument();
  expect(screen.getByRole("link", { name: "新增题目" })).toHaveAttribute("href", "/questions/new");
  expect(screen.queryByRole("button", { name: "新增标签" })).not.toBeInTheDocument();
  expect(screen.queryByLabelText("题目标签列表")).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "导入题目" })).toHaveClass("tenant-create-button");
  expect(screen.getByLabelText("搜索题目")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "刷新题目列表" })).toBeInTheDocument();
  expect(screen.queryByLabelText("题目导入文件")).not.toBeInTheDocument();
  expect(await screen.findByText((content) => content.endsWith("..."))).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "ID" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "题干" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "难度" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "题目类型" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "题目状态" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "出题人" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "所属空间" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "出题时间" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "操作区" })).toBeInTheDocument();
  expect(screen.getByRole("cell", { name: "100" })).toBeInTheDocument();
  expect(screen.getByRole("cell", { name: "中等" })).toBeInTheDocument();
  expect(screen.getByRole("cell", { name: "单选题" })).toBeInTheDocument();
  expect(screen.getByRole("cell", { name: "可用" })).toBeInTheDocument();
  expect(screen.getByRole("cell", { name: "已禁用" })).toBeInTheDocument();
  expect(screen.getByRole("cell", { name: "草稿" })).toBeInTheDocument();
  expect(screen.getByRole("cell", { name: "teacher01" })).toBeInTheDocument();
  expect(screen.getAllByRole("cell", { name: "公共题库" })).toHaveLength(3);
  expect(screen.getByRole("cell", { name: "2023-11-15 06:13" })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: `编辑 ${longQuestionTitle}` })).toHaveAttribute("href", "/questions/100/edit");
  expect(screen.getByRole("button", { name: `删除 ${longQuestionTitle}` })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: `禁用 ${longQuestionTitle}` })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "启用 草稿状态题目" })).toBeInTheDocument();
  expect(screen.getByText("共 43 条")).toBeInTheDocument();
  expect(screen.getByText("第 1 / 3 页")).toBeInTheDocument();
  expect(api.listQuestions).toHaveBeenCalledWith({ tenantID: 10, page: 1, pageSize: 20, search: "" });
});

test("题库列表展示所属空间并在教师视角允许管理公共题库操作入口", async () => {
  const api = createQuestionAPI();
  vi.mocked(api.listQuestions).mockResolvedValueOnce({
    page: 1,
    pageSize: 20,
    total: 2,
    items: [
      {
        id: 200,
        tenantID: 10,
        type: "short_text",
        title: "公共题库试题",
        stem: "公共题库试题",
        options: [],
        analysis: "公共题库题目。",
        difficulty: "medium",
        tag: "公共",
        tags: ["公共"],
        scoreDefault: "4",
        authorName: "tenant.admin",
        authorRole: "tenant_admin",
        createdAt: 1700000000000,
        status: "ready",
      },
      {
        id: 201,
        tenantID: 10,
        spaceID: 301,
        type: "short_text",
        title: "空间题库试题",
        stem: "空间题库试题",
        options: [],
        analysis: "空间题库题目。",
        difficulty: "medium",
        tag: "空间",
        tags: ["空间"],
        scoreDefault: "4",
        authorName: "teacher01",
        authorRole: "teacher",
        createdAt: 1700000000000,
        status: "ready",
      },
    ],
  });

  renderWithFeedback(<QuestionBankPage api={api} actorRole="teacher" tenantID={10} spaceID={301} spaceName="青藤一中" />);

  expect(await screen.findByRole("cell", { name: "公共题库" })).toBeInTheDocument();
  expect(screen.getByRole("cell", { name: "青藤一中" })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "编辑 公共题库试题" })).toHaveAttribute("href", "/questions/200/edit");
  expect(screen.getByRole("button", { name: "禁用 公共题库试题" })).toBeEnabled();
  expect(screen.getByRole("button", { name: "删除 公共题库试题" })).toBeEnabled();
  expect(screen.getByRole("link", { name: "编辑 空间题库试题" })).toHaveAttribute(
    "href",
    "/questions/201/edit",
  );
  expect(screen.getByRole("button", { name: "禁用 空间题库试题" })).toBeEnabled();
  expect(screen.getByRole("button", { name: "删除 空间题库试题" })).toBeEnabled();
});

test("题库列表展示所属空间并在空间管理员视角置灰公共题库操作入口", async () => {
  const api = createQuestionAPI();
  vi.mocked(api.listQuestions).mockResolvedValueOnce({
    page: 1,
    pageSize: 20,
    total: 1,
    items: [
      {
        id: 200,
        tenantID: 10,
        type: "short_text",
        title: "公共题库试题",
        stem: "公共题库试题",
        options: [],
        analysis: "公共题库题目。",
        difficulty: "medium",
        tag: "公共",
        tags: ["公共"],
        scoreDefault: "4",
        authorName: "tenant.admin",
        authorRole: "tenant_admin",
        createdAt: 1700000000000,
        status: "ready",
      },
    ],
  });

  renderWithFeedback(<QuestionBankPage api={api} actorRole="space_admin" tenantID={10} spaceID={301} spaceName="青藤一中" />);

  expect(await screen.findByRole("cell", { name: "公共题库" })).toBeInTheDocument();
  expect(screen.getByLabelText("编辑 公共题库试题")).toHaveAttribute("aria-disabled", "true");
  expect(screen.getByRole("button", { name: "禁用 公共题库试题" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "删除 公共题库试题" })).toBeDisabled();
});

test("教师可以搜索和刷新题目列表", async () => {
  const user = userEvent.setup();
  const api = createQuestionAPI();
  renderWithFeedback(<QuestionBankPage api={api} tenantID={10} />);

  await screen.findByText((content) => content.endsWith("..."));
  vi.mocked(api.listQuestions).mockResolvedValueOnce({
    page: 1,
    pageSize: 20,
    total: 1,
    items: [{
      id: 103,
      tenantID: 10,
      type: "short_text",
      title: "语言文字运用题",
      stem: "语言文字运用题",
      options: [],
      analysis: "考查语病修改。",
      difficulty: "medium",
      tag: "语言文字",
      tags: ["语言文字"],
      scoreDefault: "6",
      authorName: "teacher01",
      authorRole: "teacher",
      createdAt: 1700000180000,
      status: "ready",
    }],
  });

  await user.type(screen.getByLabelText("搜索题目"), "语言文字");
  await user.click(screen.getByRole("button", { name: "搜索" }));

  await waitFor(() => {
    expect(api.listQuestions).toHaveBeenLastCalledWith({ tenantID: 10, page: 1, pageSize: 20, search: "语言文字" });
  });
  expect(await screen.findByText("语言文字运用题")).toBeInTheDocument();

  const refreshResult = deferred<Awaited<ReturnType<QuestionAPI["listQuestions"]>>>();
  vi.mocked(api.listQuestions).mockReturnValueOnce(refreshResult.promise);

  await user.click(screen.getByRole("button", { name: "刷新题目列表" }));
  expect(screen.getByRole("button", { name: "刷新题目列表" }).querySelector("svg")).toHaveClass(
    "tenant-refresh-icon--spinning",
  );
  expect(api.listQuestions).toHaveBeenLastCalledWith({ tenantID: 10, page: 1, pageSize: 20, search: "" });

  refreshResult.resolve({
    page: 1,
    pageSize: 20,
    total: 43,
    items: [{
      id: 100,
      tenantID: 10,
      type: "single",
      title: longQuestionTitle,
      stem: "下列选项最能概括文章中心的是哪一项？",
      options: ["把握中心句", "复述细节"],
      analysis: "定位中心句并排除以偏概全选项。",
      difficulty: "medium",
      tag: "阅读理解",
      tags: ["阅读理解"],
      scoreDefault: "4",
      authorName: "teacher01",
      authorRole: "teacher",
      createdAt: 1700000000000,
      status: "ready",
    }],
  });

  expect(await screen.findByText((content) => content.endsWith("..."))).toBeInTheDocument();
});

test("刷新题库列表会清空搜索并回到第一页", async () => {
  const user = userEvent.setup();
  const api = createQuestionAPI();
  renderWithFeedback(<QuestionBankPage api={api} tenantID={10} />);

  await screen.findByText((content) => content.endsWith("..."));
  vi.mocked(api.listQuestions).mockResolvedValueOnce({
    page: 2,
    pageSize: 20,
    total: 43,
    items: [{
      id: 201,
      tenantID: 10,
      type: "single",
      title: "第二页题目",
      stem: "第二页题目",
      options: ["A", "B"],
      analysis: "用于验证分页刷新。",
      difficulty: "medium",
      tag: "阅读理解",
      tags: ["阅读理解"],
      scoreDefault: "4",
      authorName: "teacher01",
      authorRole: "teacher",
      createdAt: 1700000000000,
      status: "ready",
    }],
  });
  await user.click(screen.getByRole("button", { name: "下一页" }));
  await waitFor(() => {
    expect(api.listQuestions).toHaveBeenLastCalledWith({ tenantID: 10, page: 2, pageSize: 20, search: "" });
  });

  await user.type(screen.getByLabelText("搜索题目"), "teacher01");
  const refreshResult = deferred<Awaited<ReturnType<QuestionAPI["listQuestions"]>>>();
  vi.mocked(api.listQuestions).mockReturnValueOnce(refreshResult.promise);
  await user.click(screen.getByRole("button", { name: "刷新题目列表" }));

  expect(api.listQuestions).toHaveBeenLastCalledWith({ tenantID: 10, page: 1, pageSize: 20, search: "" });
  expect(screen.getByLabelText("搜索题目")).toHaveValue("");
  refreshResult.resolve({
    page: 1,
    pageSize: 20,
    total: 43,
    items: [],
  });
});

test("题库搜索结果会重置分页显示为当前过滤结果", async () => {
  const user = userEvent.setup();
  const api = createQuestionAPI();
  renderWithFeedback(<QuestionBankPage api={api} tenantID={10} />);

  await screen.findByText((content) => content.endsWith("..."));
  vi.mocked(api.listQuestions).mockResolvedValueOnce({
    page: 1,
    pageSize: 20,
    total: 21,
    items: [{
      id: 200,
      tenantID: 10,
      type: "single",
      title: "teacher01 后页题目",
      stem: "teacher01 后页题目",
      options: ["A", "B"],
      analysis: "用于验证搜索由服务端分页返回。",
      difficulty: "medium",
      tag: "阅读理解",
      tags: ["阅读理解"],
      scoreDefault: "4",
      authorName: "teacher01",
      authorRole: "teacher",
      createdAt: 1700000000000,
      status: "ready",
    }],
  });
  await user.type(screen.getByLabelText("搜索题目"), "teacher01");
  await user.click(screen.getByRole("button", { name: "搜索" }));

  await waitFor(() => {
    expect(api.listQuestions).toHaveBeenLastCalledWith({ tenantID: 10, page: 1, pageSize: 20, search: "teacher01" });
  });
  expect(screen.getByText("共 21 条")).toBeInTheDocument();
  expect(screen.getByText("第 1 / 2 页")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "上一页" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "下一页" })).not.toBeDisabled();
});

test("题库分页支持切换页码和每页条数", async () => {
  const user = userEvent.setup();
  const api = createQuestionAPI();
  renderWithFeedback(<QuestionBankPage api={api} tenantID={10} />);

  await screen.findByText((content) => content.endsWith("..."));
  expect(screen.getByText("第 1 / 3 页")).toBeInTheDocument();

  vi.mocked(api.listQuestions).mockResolvedValueOnce({
    page: 2,
    pageSize: 20,
    total: 43,
    items: [],
  });
  await user.click(screen.getByRole("button", { name: "下一页" }));

  expect(api.listQuestions).toHaveBeenLastCalledWith({ tenantID: 10, page: 2, pageSize: 20, search: "" });

  vi.mocked(api.listQuestions).mockResolvedValueOnce({
    page: 1,
    pageSize: 50,
    total: 43,
    items: [],
  });
  await user.click(screen.getByRole("combobox", { name: "每页条数" }));
  await user.click(screen.getByRole("option", { name: "50 条 / 页" }));

  expect(api.listQuestions).toHaveBeenLastCalledWith({ tenantID: 10, page: 1, pageSize: 50, search: "" });
});

test("新增和编辑题目使用前端路由跳转", async () => {
  const user = userEvent.setup();
  const api = createQuestionAPI();
  const { unmount } = renderQuestionBankRoutes(api);

  await screen.findByText((content) => content.endsWith("..."));
  await user.click(screen.getByRole("link", { name: "新增题目" }));

  expect(screen.getByLabelText("当前路由")).toHaveTextContent("/questions/new?space_id=301");

  unmount();
  renderQuestionBankRoutes(createQuestionAPI());
  await screen.findByText((content) => content.endsWith("..."));
  await user.click(screen.getByRole("link", { name: `编辑 ${longQuestionTitle}` }));

  expect(screen.getByLabelText("当前路由")).toHaveTextContent("/questions/100/edit?space_id=301");
});

test("教师可以在题库列表禁用和删除题目，并展示引用校验失败", async () => {
  const user = userEvent.setup();
  const api = createQuestionAPI();
  vi.mocked(api.deleteQuestion).mockRejectedValueOnce(
    new ApiError("题目已被试卷或考试引用，不能删除", 40001, 409, {}),
  );
  renderWithFeedback(<QuestionBankPage api={api} tenantID={10} />);

  await screen.findByText((content) => content.endsWith("..."));
  await user.click(screen.getByRole("button", { name: `禁用 ${longQuestionTitle}` }));

  expect(api.disableQuestion).toHaveBeenCalledWith({ tenantID: 10, questionID: 100 });
  const successAlert = await screen.findByRole("alert");
  expect(successAlert).toHaveTextContent("题目已禁用");
  expect(successAlert.closest(".ant-message")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: `启用 ${longQuestionTitle}` })).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: `删除 ${longQuestionTitle}` }));

  expect(api.deleteQuestion).toHaveBeenCalledWith({ tenantID: 10, questionID: 100 });
  const errorText = await screen.findByText("题目已被试卷或考试引用，不能删除");
  expect(errorText.closest(".ant-message")).toBeInTheDocument();
  expect(document.querySelector(".tenant-admin-warning")).not.toBeInTheDocument();
  expect(document.querySelector(".tenant-admin-success")).not.toBeInTheDocument();
});

test("教师可以在题库右侧抽屉导入题目", async () => {
  const user = userEvent.setup();
  const api = createQuestionAPI();
  renderWithFeedback(<QuestionBankPage api={api} tenantID={10} spaceID={301} />);

  await screen.findByText((content) => content.endsWith("..."));

  await user.click(screen.getByRole("button", { name: "导入题目" }));

  const drawer = screen.getByRole("dialog", { name: "题目导入抽屉" });
  expect(drawer.closest(".ant-drawer")).toBeInTheDocument();
  expect(drawer).toHaveClass("tenant-resource-drawer--open");
  expect(within(drawer).queryByLabelText("题目导入文件预览")).not.toBeInTheDocument();
  const templateLink = within(drawer).getByRole("link", { name: "下载 CSV 模板" });
  expect(templateLink).toHaveAttribute("download", "question-import-template.csv");
  expect(templateLink).toHaveAttribute("href", expect.stringContaining("data:text/csv"));
  expect(within(drawer).getByLabelText("导入所属空间")).toHaveValue("space:301");
  expect(within(drawer).getByLabelText("导入所属空间").closest(".question-import-scope-field")).not.toBeNull();
  expect(within(drawer).getByLabelText("题目导入文件")).toHaveAttribute("accept", "text/csv");
  expect(within(drawer).queryByRole("combobox", { name: "导入后题目状态" })).not.toBeInTheDocument();
  expect(within(drawer).getByRole("radio", { name: "草稿" })).toBeChecked();
  expect(within(drawer).getByRole("radio", { name: "已启用" })).not.toBeChecked();

  const file = new File(["type,title"], "questions.csv", { type: "text/csv" });
  await user.upload(within(drawer).getByLabelText("题目导入文件"), file);
  expect(within(drawer).getAllByText("questions.csv").length).toBeGreaterThan(0);
  expect(api.startQuestionImportJob).not.toHaveBeenCalled();

  await user.click(within(drawer).getByRole("button", { name: "确认导入" }));

  expect(api.startQuestionImportJob).toHaveBeenCalledWith({ tenantID: 10, spaceID: 301, file, status: "draft" });
  expect(api.subscribeQuestionImportJob).toHaveBeenCalledWith({ jobID: "job-1" }, expect.any(Function), expect.any(Function));
  const alert = await screen.findByRole("alert");
  expect(alert).toHaveTextContent("导入完成 2 条");
  expect(alert.closest(".ant-message")).toBeInTheDocument();
  expect(within(drawer).getAllByText("questions.csv").length).toBeGreaterThan(0);
});

test("题库导入抽屉可以选择导入到公共题库", async () => {
  const user = userEvent.setup();
  const api = createQuestionAPI();
  renderWithFeedback(<QuestionBankPage api={api} tenantID={10} spaceID={301} />);

  await screen.findByText((content) => content.endsWith("..."));
  await user.click(screen.getByRole("button", { name: "导入题目" }));

  const drawer = screen.getByRole("dialog", { name: "题目导入抽屉" });
  await user.selectOptions(within(drawer).getByLabelText("导入所属空间"), "public");
  const file = new File(["type,title"], "public-questions.csv", { type: "text/csv" });
  await user.upload(within(drawer).getByLabelText("题目导入文件"), file);
  await user.click(within(drawer).getByRole("button", { name: "确认导入" }));

  expect(api.startQuestionImportJob).toHaveBeenCalledWith({ tenantID: 10, spaceID: null, file, status: "draft" });
});

test("租户管理员可以在题库导入抽屉选择导入到具体空间且导入后刷新当前列表范围", async () => {
  const user = userEvent.setup();
  const api = createQuestionAPI();
  const spaceApi = createSpaceAPI();
  renderWithFeedback(
    <QuestionBankPage actorRole="tenant_admin" api={api} spaceApi={spaceApi} tenantID={10} />,
    "/questions?space_id=301",
  );

  await screen.findByText((content) => content.endsWith("..."));
  await user.click(screen.getByRole("button", { name: "导入题目" }));

  const drawer = screen.getByRole("dialog", { name: "题目导入抽屉" });
  await within(drawer).findByRole("option", { name: "高一二班" });
  expect(spaceApi.listSpaces).toHaveBeenCalledWith(10);
  await user.selectOptions(within(drawer).getByLabelText("导入所属空间"), "space:302");

  const file = new File(["type,title"], "tenant-space-questions.csv", { type: "text/csv" });
  await user.upload(within(drawer).getByLabelText("题目导入文件"), file);
  await user.click(within(drawer).getByRole("button", { name: "确认导入" }));

  expect(api.startQuestionImportJob).toHaveBeenCalledWith({ tenantID: 10, spaceID: 302, file, status: "draft" });
  await waitFor(() => {
    expect(api.listQuestions).toHaveBeenLastCalledWith({
      tenantID: 10,
      spaceID: 301,
      page: 1,
      pageSize: 20,
      search: "",
    });
  });
});

test("空间管理员在题库导入抽屉中看不到公共题库选项", async () => {
  const user = userEvent.setup();
  const api = createQuestionAPI();
  renderQuestionBankRoutesWithRole(api, "space_admin");

  await screen.findByText((content) => content.endsWith("..."));
  await user.click(screen.getByRole("button", { name: "导入题目" }));

  const drawer = screen.getByRole("dialog", { name: "题目导入抽屉" });
  expect(within(drawer).queryByRole("option", { name: "公共题库" })).not.toBeInTheDocument();
  expect(within(drawer).getByRole("option", { name: "当前空间题库" })).toBeInTheDocument();
});

test("题目导入抽屉支持多文件队列并依次启动导入任务", async () => {
  const user = userEvent.setup();
  const api = createQuestionAPI();
  vi.mocked(api.startQuestionImportJob)
    .mockResolvedValueOnce({ jobID: "job-1" })
    .mockResolvedValueOnce({ jobID: "job-2" });
  vi.mocked(api.subscribeQuestionImportJob)
    .mockImplementationOnce((_input, onEvent) => {
      onEvent({
        jobID: "job-1",
        status: "completed",
        fileName: "first.csv",
        totalRows: 3,
        processedRows: 3,
        successCount: 2,
        errorCount: 0,
        duplicateCount: 1,
        errors: [],
      });
      return () => undefined;
    })
    .mockImplementationOnce((_input, onEvent) => {
      onEvent({
        jobID: "job-2",
        status: "completed",
        fileName: "second.csv",
        totalRows: 1,
        processedRows: 1,
        successCount: 1,
        errorCount: 0,
        duplicateCount: 0,
        errors: [],
      });
      return () => undefined;
    });
  renderWithFeedback(<QuestionBankPage api={api} tenantID={10} spaceID={301} />);

  await screen.findByText((content) => content.endsWith("..."));
  await user.click(screen.getByRole("button", { name: "导入题目" }));
  const drawer = screen.getByRole("dialog", { name: "题目导入抽屉" });
  await user.click(within(drawer).getByRole("radio", { name: "已启用" }));
  expect(within(drawer).getByRole("radio", { name: "已启用" })).toBeChecked();
  const firstFile = new File(["type,title"], "first.csv", { type: "text/csv" });
  const secondFile = new File(["type,title"], "second.csv", { type: "text/csv" });

  await user.upload(within(drawer).getByLabelText("题目导入文件"), [firstFile, secondFile]);

  expect(within(drawer).getByText("first.csv")).toBeInTheDocument();
  expect(within(drawer).getByText("second.csv")).toBeInTheDocument();
  expect(within(drawer).getAllByRole("progressbar")).toHaveLength(2);

  await user.click(within(drawer).getByRole("button", { name: "确认导入" }));

  await waitFor(() => {
    expect(api.startQuestionImportJob).toHaveBeenNthCalledWith(1, { tenantID: 10, spaceID: 301, file: firstFile, status: "enabled" });
    expect(api.startQuestionImportJob).toHaveBeenNthCalledWith(2, { tenantID: 10, spaceID: 301, file: secondFile, status: "enabled" });
  });
  expect(within(drawer).getByText("成功 2 / 失败 0 / 重复 1")).toBeInTheDocument();
  expect(within(drawer).getByText("成功 1 / 失败 0 / 重复 0")).toBeInTheDocument();
});

test("导入任务启动失败后保留失败状态并允许重试", async () => {
  const user = userEvent.setup();
  const api = createQuestionAPI();
  vi.mocked(api.startQuestionImportJob)
    .mockRejectedValueOnce(new Error("上传连接失败"))
    .mockResolvedValueOnce({ jobID: "job-retry" });
  vi.mocked(api.subscribeQuestionImportJob).mockImplementationOnce((_input, onEvent) => {
    onEvent({
      jobID: "job-retry",
      status: "completed",
      fileName: "questions.csv",
      totalRows: 1,
      processedRows: 1,
      successCount: 1,
      errorCount: 0,
      duplicateCount: 0,
      errors: [],
    });
    return () => undefined;
  });
  renderWithFeedback(<QuestionBankPage api={api} tenantID={10} spaceID={301} />);

  await screen.findByText((content) => content.endsWith("..."));
  await user.click(screen.getByRole("button", { name: "导入题目" }));
  const drawer = screen.getByRole("dialog", { name: "题目导入抽屉" });
  const file = new File(["type,title"], "questions.csv", { type: "text/csv" });
  await user.upload(within(drawer).getByLabelText("题目导入文件"), file);

  await user.click(within(drawer).getByRole("button", { name: "确认导入" }));
  expect(await within(drawer).findByText("上传连接失败")).toBeInTheDocument();

  await user.click(within(drawer).getByRole("button", { name: "确认导入" }));

  await waitFor(() => {
    expect(api.startQuestionImportJob).toHaveBeenCalledTimes(2);
  });
  expect(within(drawer).getByText("成功 1 / 失败 0 / 重复 0")).toBeInTheDocument();
});

test("搜索后导入题目会按当前搜索条件刷新列表", async () => {
  const user = userEvent.setup();
  const api = createQuestionAPI();
  renderWithFeedback(<QuestionBankPage api={api} tenantID={10} spaceID={301} />);

  await screen.findByText((content) => content.endsWith("..."));
  vi.mocked(api.listQuestions).mockResolvedValueOnce({
    page: 1,
    pageSize: 20,
    total: 1,
    items: [{
      id: 103,
      tenantID: 10,
      type: "short_text",
      title: "语言文字运用题",
      stem: "语言文字运用题",
      options: [],
      analysis: "考查语病修改。",
      difficulty: "medium",
      tag: "语言文字",
      tags: ["语言文字"],
      scoreDefault: "6",
      authorName: "teacher01",
      authorRole: "teacher",
      createdAt: 1700000180000,
      status: "ready",
    }],
  });

  await user.type(screen.getByLabelText("搜索题目"), "语言文字");
  await user.click(screen.getByRole("button", { name: "搜索" }));
  await waitFor(() => {
    expect(api.listQuestions).toHaveBeenLastCalledWith({
      tenantID: 10,
      spaceID: 301,
      page: 1,
      pageSize: 20,
      search: "语言文字",
    });
  });

  await user.click(screen.getByRole("button", { name: "导入题目" }));
  const drawer = screen.getByRole("dialog", { name: "题目导入抽屉" });
  const file = new File(["type,title"], "questions.csv", { type: "text/csv" });
  await user.upload(within(drawer).getByLabelText("题目导入文件"), file);

  vi.mocked(api.listQuestions).mockResolvedValueOnce({
    page: 1,
    pageSize: 20,
    total: 1,
    items: [{
      id: 104,
      tenantID: 10,
      type: "short_text",
      title: "新导入的语言文字题",
      stem: "新导入的语言文字题",
      options: [],
      analysis: "导入后仍按当前搜索展示。",
      difficulty: "medium",
      tag: "语言文字",
      tags: ["语言文字"],
      scoreDefault: "6",
      authorName: "teacher01",
      authorRole: "teacher",
      createdAt: 1700000240000,
      status: "ready",
    }],
  });
  await user.click(within(drawer).getByRole("button", { name: "确认导入" }));

  await waitFor(() => {
    expect(api.listQuestions).toHaveBeenLastCalledWith({
      tenantID: 10,
      spaceID: 301,
      page: 1,
      pageSize: 20,
      search: "语言文字",
    });
  });
  expect(screen.getByLabelText("搜索题目")).toHaveValue("语言文字");
});

test("长题干在悬停时通过 tooltip 展示完整内容", async () => {
  const user = userEvent.setup();
  const api = createQuestionAPI();
  renderWithFeedback(<QuestionBankPage api={api} tenantID={10} />);

  const truncatedTitle = await screen.findByText((content) => content.endsWith("..."));
  await user.hover(truncatedTitle);

  const tooltip = await screen.findByRole("tooltip");
  expect(tooltip).toHaveTextContent(longQuestionTitle);
  expect(tooltip).toHaveClass("ant-tooltip-container");
});

test("题库列表加载和导入校验失败时使用 toast 提示", async () => {
  const user = userEvent.setup();
  const api = createQuestionAPI();
  api.listQuestions.mockRejectedValueOnce(new Error("load failed"));
  renderWithFeedback(<QuestionBankPage api={api} tenantID={10} />);

  const loadAlert = await screen.findByRole("alert");
  expect(loadAlert).toHaveTextContent("题目列表加载失败");
  expect(loadAlert.closest(".ant-message")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "导入题目" }));
  await user.click(screen.getByRole("button", { name: "确认导入" }));

  expect(await screen.findByText("请选择题目导入文件")).toBeInTheDocument();
  expect(document.querySelector(".tenant-admin-warning")).not.toBeInTheDocument();
  expect(document.querySelector(".tenant-admin-success")).not.toBeInTheDocument();
});
