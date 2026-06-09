import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { expect, test, vi } from "vitest";
import type { ActorRole } from "../../api/grading";
import { FeedbackProvider } from "../../app/feedback";
import type { SpaceManagementAPI } from "../../api/spaces";
import { QuestionCreatePage } from "./QuestionCreatePage";

function renderWithFeedback(page: ReactElement) {
  return render(<FeedbackProvider>{page}</FeedbackProvider>);
}

function createQuestionAPI() {
  return {
    listQuestions: vi.fn().mockResolvedValue({
      page: 1,
      pageSize: 20,
      total: 1,
      items: [
        {
          id: 100,
          tenantID: 10,
          spaceID: 301,
          type: "single",
          title: "现代文阅读主旨题",
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
      ],
    }),
    createQuestion: vi.fn().mockResolvedValue({
      id: 101,
      tenantID: 10,
      spaceID: 301,
      type: "multiple",
      title: "下列函数在 R 上单调递增的是哪一项？",
      stem: "下列函数在 R 上单调递增的是哪一项？",
      options: ["y = x", "y = -x", "y = x + 1"],
      analysis: "一次函数斜率为正时单调递增。",
      difficulty: "hard",
      tag: "函数",
      tags: ["阅读理解", "函数"],
      scoreDefault: "6",
      authorName: "teacher01",
      authorRole: "teacher",
      createdAt: 1700000000000,
      status: "draft",
    }),
    getQuestion: vi.fn().mockResolvedValue({
      id: 100,
      tenantID: 10,
      spaceID: 301,
      type: "single",
      title: "原题干",
      stem: "原题干",
      options: ["原选项 A", "原选项 B"],
      correctOptionIndexes: [0],
      analysis: "原解析",
      difficulty: "easy",
      tag: "阅读理解",
      tags: ["阅读理解"],
      scoreDefault: "4",
      authorName: "teacher01",
      authorRole: "teacher",
      createdAt: 1700000000000,
      status: "ready",
    }),
    updateQuestion: vi.fn().mockResolvedValue({
      id: 100,
      tenantID: 10,
      spaceID: 301,
      type: "single",
      title: "更新后题干",
      stem: "更新后题干",
      options: ["原选项 A", "原选项 B"],
      correctOptionIndexes: [0],
      analysis: "更新后解析",
      difficulty: "easy",
      tag: "阅读理解",
      tags: ["阅读理解"],
      scoreDefault: "4",
      authorName: "teacher01",
      authorRole: "teacher",
      createdAt: 1700000000000,
      status: "ready",
    }),
    deleteQuestion: vi.fn(),
    disableQuestion: vi.fn(),
    enableQuestion: vi.fn(),
    importQuestions: vi.fn(),
    startQuestionImportJob: vi.fn(),
    subscribeQuestionImportJob: vi.fn(),
    listQuestionTags: vi.fn().mockResolvedValue(["阅读理解"]),
    countAvailableQuestions: vi.fn().mockResolvedValue({}),
  };
}

function renderCreatePage(api = createQuestionAPI()) {
  renderWithFeedback(
    <MemoryRouter initialEntries={["/questions/new?space_id=301"]}>
      <Routes>
        <Route path="/questions/new" element={<QuestionCreatePage api={api} tenantID={10} spaceID={301} />} />
        <Route path="/questions" element={<div>题库列表页</div>} />
      </Routes>
    </MemoryRouter>,
  );
  return api;
}

function renderCreatePageWithRole(actorRole: ActorRole, api = createQuestionAPI()) {
  renderWithFeedback(
    <MemoryRouter initialEntries={["/questions/new?space_id=301"]}>
      <Routes>
        <Route path="/questions/new" element={<QuestionCreatePage actorRole={actorRole} api={api} tenantID={10} spaceID={301} />} />
        <Route path="/questions" element={<div>题库列表页</div>} />
      </Routes>
    </MemoryRouter>,
  );
  return api;
}

function createSpaceAPI(): Pick<SpaceManagementAPI, "listSpaces"> {
  const spaces = [
    {
      id: 301,
      tenantID: 10,
      name: "高一一班",
      description: "高一一班空间",
      logoFileName: "class-a.png",
      status: "enabled" as const,
      members: [],
    },
    {
      id: 302,
      tenantID: 10,
      name: "高一二班",
      description: "高一二班空间",
      logoFileName: "class-b.png",
      status: "enabled" as const,
      members: [],
    },
    {
      id: 303,
      tenantID: 10,
      name: "已禁用班级",
      description: "禁用空间",
      logoFileName: "class-disabled.png",
      status: "disabled" as const,
      members: [],
    },
  ];
  return {
    listSpaces: vi.fn().mockImplementation(async (input) => {
      const items = input.filters?.status === undefined ? spaces : spaces.filter((space) => space.status === input.filters?.status);
      return { items, total: items.length };
    }),
  };
}

function renderTenantAdminCreatePage(api = createQuestionAPI(), spacesApi = createSpaceAPI()) {
  renderWithFeedback(
    <MemoryRouter initialEntries={["/questions/new"]}>
      <Routes>
        <Route
          path="/questions/new"
          element={<QuestionCreatePage actorRole="tenant_admin" api={api} spaceApi={spacesApi} tenantID={10} />}
        />
        <Route path="/questions" element={<div>题库列表页</div>} />
      </Routes>
    </MemoryRouter>,
  );
  return { api, spacesApi };
}

function renderEditPage(api = createQuestionAPI()) {
  renderWithFeedback(
    <MemoryRouter initialEntries={["/questions/100/edit?space_id=301"]}>
      <Routes>
        <Route path="/questions/:questionID/edit" element={<QuestionCreatePage api={api} tenantID={10} spaceID={301} questionID={100} />} />
        <Route path="/questions" element={<div>题库列表页</div>} />
      </Routes>
    </MemoryRouter>,
  );
  return api;
}

test("新增题目使用独立页面并通过插件渲染 Markdown 预览", async () => {
  const user = userEvent.setup();
  renderCreatePage();

  expect(screen.getByRole("tab", { name: "新增题目" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByRole("link", { name: "返回题库" })).toHaveClass("exam-question-create-head__back");
  expect(screen.getByText("|")).toHaveClass("exam-question-create-head__divider");
  expect(screen.getByRole("heading", { name: "新增题目" })).toHaveClass("exam-question-create-head__title");
  expect(document.querySelectorAll(".w-md-editor").length).toBeGreaterThanOrEqual(2);
  const previewPanels = document.querySelectorAll(".w-md-editor-preview");
  expect(previewPanels.length).toBeGreaterThanOrEqual(2);
  expect(document.querySelector(".w-md-editor")).toHaveStyle({ height: "auto" });
  expect(document.querySelector(".markdown-editor__preview")).not.toBeInTheDocument();
  expect(screen.queryByText("题干和解析支持 Markdown，编辑时会即时渲染预览。")).not.toBeInTheDocument();
  const stemEditor = screen.getByLabelText("题干").closest(".markdown-editor");
  expect(stemEditor).not.toBeNull();
  expect(within(stemEditor as HTMLElement).getByTitle("加粗（Ctrl+B）")).toBeInTheDocument();
  expect(within(stemEditor as HTMLElement).getByTitle("标题")).toBeInTheDocument();
  expect(within(stemEditor as HTMLElement).getByTitle("实时预览（Ctrl+8）")).toBeInTheDocument();

  await user.type(screen.getByLabelText("题干"), "## 函数题\n**单调递增**");
  await user.type(screen.getByLabelText("题目解析"), "- 看斜率\n- 排除反例");

  const stemPreview = within(previewPanels[0] as HTMLElement);
  const analysisPreview = within(previewPanels[1] as HTMLElement);
  expect(stemPreview.getByRole("heading", { name: "函数题" })).toBeInTheDocument();
  expect(stemPreview.getByText("单调递增")).toBeInTheDocument();
  expect(analysisPreview.getByText("看斜率")).toBeInTheDocument();
  expect(analysisPreview.getByText("排除反例")).toBeInTheDocument();
});

test("题干 Markdown 预览支持渲染 LaTeX 公式", async () => {
  renderCreatePage();

  fireEvent.change(screen.getByLabelText("题干"), {
    target: { value: "行内公式 $a^2+b^2=c^2$\n\n$$\\sum_{i=1}^{n} i$$" },
  });

  const previewPanels = document.querySelectorAll(".w-md-editor-preview");
  expect(previewPanels.length).toBeGreaterThanOrEqual(1);
  await waitFor(() => {
    expect(previewPanels[0]?.querySelector(".katex")).not.toBeNull();
  });
});

test("新增简答题参考答案使用 Markdown 编辑器并保存原文", async () => {
  const user = userEvent.setup();
  const api = renderCreatePage();
  const referenceAnswer = "## 参考答案\n- 采分点一\n- **采分点二**";

  await user.selectOptions(screen.getByLabelText("题型"), "short_text");
  fireEvent.change(screen.getByLabelText("题干"), { target: { value: "请简述函数单调性的判断方法。" } });
  fireEvent.change(screen.getByLabelText("题目解析"), { target: { value: "从定义和图像两个角度分析。" } });
  fireEvent.change(screen.getByLabelText("简答题参考答案"), { target: { value: referenceAnswer } });

  const referenceEditor = screen.getByLabelText("简答题参考答案").closest(".markdown-editor");
  expect(referenceEditor).not.toBeNull();
  expect(referenceEditor?.querySelector(".w-md-editor")).toBeInTheDocument();
  await waitFor(() => {
    expect(within(referenceEditor as HTMLElement).getByRole("heading", { name: "参考答案" })).toBeInTheDocument();
  });
  expect(within(referenceEditor as HTMLElement).getByText("采分点一")).toBeInTheDocument();
  expect(within(referenceEditor as HTMLElement).getByText("采分点二")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "确认新增" }));

  expect(api.createQuestion).toHaveBeenCalledWith(expect.objectContaining({
    type: "short_text",
    title: "请简述函数单调性的判断方法。",
    options: [],
    correctOptionIndexes: [],
    referenceAnswer,
    standardAnswer: undefined,
  }));
});

test("新增简答题时参考答案不能为空", async () => {
  const user = userEvent.setup();
  const api = renderCreatePage();

  await user.selectOptions(screen.getByLabelText("题型"), "short_text");
  fireEvent.change(screen.getByLabelText("题干"), { target: { value: "请简述函数单调性的判断方法。" } });
  fireEvent.change(screen.getByLabelText("题目解析"), { target: { value: "从定义和图像两个角度分析。" } });
  await user.click(screen.getByRole("button", { name: "确认新增" }));

  expect(api.createQuestion).not.toHaveBeenCalled();
  expect(await screen.findByRole("alert")).toHaveTextContent("简答题参考答案不能为空");
});

test("新增题目时默认分值不能为空", async () => {
  const user = userEvent.setup();
  const api = renderCreatePage();

  await user.clear(screen.getByLabelText("默认分值"));
  fireEvent.change(screen.getByLabelText("题干"), { target: { value: "默认分值为空的题目" } });
  fireEvent.change(screen.getByLabelText("选项 A"), { target: { value: "选项 A" } });
  fireEvent.change(screen.getByLabelText("选项 B"), { target: { value: "选项 B" } });
  fireEvent.change(screen.getByLabelText("题目解析"), { target: { value: "默认分值不能为空。" } });
  await user.click(screen.getByRole("button", { name: "确认新增" }));

  expect(api.createQuestion).not.toHaveBeenCalled();
  expect(await screen.findByRole("alert")).toHaveTextContent("默认分值不能为空");
});

test("教师可以在新增题目页面创建选择题并编辑选项和标签", async () => {
  const user = userEvent.setup();
  const api = renderCreatePage();

  await user.selectOptions(screen.getByLabelText("题型"), "multiple");
  const optionSection = screen.getByLabelText("选择题选项");
  expect(within(optionSection).getByText("支持公式：行内 $a^2+b^2=c^2$，块级 $$\\sum_{i=1}^{n} i$$")).toHaveClass("exam-option-list__formula-hint");
  await user.selectOptions(screen.getByLabelText("题目难度"), "hard");
  await user.clear(screen.getByLabelText("默认分值"));
  await user.type(screen.getByLabelText("默认分值"), "6");
  await user.clear(screen.getByLabelText("题干"));
  await user.type(screen.getByLabelText("题干"), "## 函数题\n下列函数在 R 上**单调递增**的是哪一项？");
  await user.clear(screen.getByLabelText("选项 A"));
  await user.type(screen.getByLabelText("选项 A"), "$y = x$");
  await user.clear(screen.getByLabelText("选项 B"));
  await user.type(screen.getByLabelText("选项 B"), "$y = -x$");
  await user.click(screen.getByRole("button", { name: "添加选项" }));
  await user.clear(screen.getByLabelText("选项 C"));
  await user.type(screen.getByLabelText("选项 C"), "$y = x + 1$");
  await user.click(screen.getByLabelText("设为正确答案 C"));
  await user.clear(screen.getByLabelText("题目解析"));
  await user.type(screen.getByLabelText("题目解析"), "- 一次函数斜率为正时单调递增。\n- 排除斜率为负的函数。");
  await user.type(screen.getByLabelText("题目标签"), "阅");
  await user.click(screen.getByRole("option", { name: /阅读理解/ }));
  await user.type(screen.getByLabelText("题目标签"), "函数");
  await user.keyboard("{Enter}");
  await user.click(screen.getByRole("button", { name: "确认新增" }));

  expect(api.createQuestion).toHaveBeenCalledWith({
    tenantID: 10,
    spaceID: 301,
    type: "multiple",
    difficulty: "hard",
    title: "## 函数题\n下列函数在 R 上**单调递增**的是哪一项？",
    options: ["$y = x$", "$y = -x$", "$y = x + 1$"],
    correctOptionIndexes: [0, 1, 2],
    analysis: "- 一次函数斜率为正时单调递增。\n- 排除斜率为负的函数。",
    scoreDefault: "6",
    tags: ["阅读理解", "函数"],
    standardAnswer: undefined,
    referenceAnswer: undefined,
    blankCount: undefined,
  });
  expect(await screen.findByText("题库列表页")).toBeInTheDocument();
});

test("新增题目时选择题选项默认置空", async () => {
  const user = userEvent.setup();
  renderCreatePage();

  expect(screen.getByLabelText("选项 A")).toHaveValue("");
  expect(screen.getByLabelText("选项 B")).toHaveValue("");

  await user.click(screen.getByRole("button", { name: "添加选项" }));

  expect(screen.getByLabelText("选项 C")).toHaveValue("");
});

test("新增题目可以选择归属到公共题库", async () => {
  const user = userEvent.setup();
  const api = renderCreatePage();

  await user.selectOptions(screen.getByLabelText("所属空间"), "public");
  await user.clear(screen.getByLabelText("题干"));
  await user.type(screen.getByLabelText("题干"), "公共题库题目");
  await user.type(screen.getByLabelText("选项 A"), "公共选项 A");
  await user.type(screen.getByLabelText("选项 B"), "公共选项 B");
  await user.clear(screen.getByLabelText("题目解析"));
  await user.type(screen.getByLabelText("题目解析"), "公共题库解析");
  await user.click(screen.getByRole("button", { name: "确认新增" }));

  expect(api.createQuestion).toHaveBeenCalledWith(expect.objectContaining({
    tenantID: 10,
    spaceID: null,
    title: "公共题库题目",
    analysis: "公共题库解析",
  }));
});

test("租户管理员新增题目时可以选择归属到具体空间", async () => {
  const user = userEvent.setup();
  const { api, spacesApi } = renderTenantAdminCreatePage();

  await waitFor(() => {
    expect(spacesApi.listSpaces).toHaveBeenCalledWith(expect.objectContaining({
      tenantID: 10,
      page: 1,
      pageSize: 100,
      filters: { status: "enabled" },
    }));
  });
  expect(screen.queryByRole("option", { name: "已禁用班级" })).not.toBeInTheDocument();
  await user.selectOptions(screen.getByLabelText("所属空间"), "space:302");
  await user.clear(screen.getByLabelText("题干"));
  await user.type(screen.getByLabelText("题干"), "空间题库题目");
  await user.type(screen.getByLabelText("选项 A"), "空间选项 A");
  await user.type(screen.getByLabelText("选项 B"), "空间选项 B");
  await user.clear(screen.getByLabelText("题目解析"));
  await user.type(screen.getByLabelText("题目解析"), "空间题库解析");
  await user.click(screen.getByRole("button", { name: "确认新增" }));

  expect(api.createQuestion).toHaveBeenCalledWith(expect.objectContaining({
    tenantID: 10,
    spaceID: 302,
    title: "空间题库题目",
    analysis: "空间题库解析",
  }));
});

test("租户管理员从后续页启用空间上下文新增题目时保留有效空间", async () => {
  const api = createQuestionAPI();
  const spacesApi = createSpaceAPI();
  const firstPageSpaces = Array.from({ length: 100 }, (_, index) => ({
    id: 300 + index,
    tenantID: 10,
    name: `第 ${index + 1} 班`,
    description: "分页空间",
    logoFileName: "class.png",
    status: "enabled" as const,
    members: [],
  }));
  vi.mocked(spacesApi.listSpaces).mockImplementation(async (input) => ({
    items: input.page === 1
      ? firstPageSpaces
      : [{
        id: 999,
        tenantID: 10,
        name: "第 101 班",
        description: "后续页空间",
        logoFileName: "class-101.png",
        status: "enabled",
        members: [],
      }],
    total: 101,
  }));

  renderWithFeedback(
    <MemoryRouter initialEntries={["/questions/new?space_id=999"]}>
      <Routes>
        <Route
          path="/questions/new"
          element={<QuestionCreatePage actorRole="tenant_admin" api={api} spaceApi={spacesApi} tenantID={10} />}
        />
        <Route path="/questions" element={<div>题库列表页</div>} />
      </Routes>
    </MemoryRouter>,
  );

  await waitFor(() => expect(spacesApi.listSpaces).toHaveBeenCalledWith(expect.objectContaining({
    tenantID: 10,
    page: 2,
    pageSize: 100,
    filters: { status: "enabled" },
  })));
  expect(screen.getByLabelText("所属空间")).toHaveValue("space:999");
  expect(screen.getByRole("option", { name: "第 101 班" })).toBeInTheDocument();
});

test("新增题目标签候选来自后端标签接口", async () => {
  const user = userEvent.setup();
  const api = createQuestionAPI();
  vi.mocked(api.listQuestions).mockResolvedValue({
    page: 1,
    pageSize: 20,
    total: 0,
    items: [],
  });
  vi.mocked(api.listQuestionTags).mockResolvedValue(["后端标签"]);

  renderCreatePage(api);

  await waitFor(() => expect(api.listQuestionTags).toHaveBeenCalledWith({ tenantID: 10, spaceID: 301 }));
  await user.type(screen.getByLabelText("题目标签"), "后");

  expect(await screen.findByRole("option", { name: /后端标签/ })).toBeInTheDocument();
});

test("租户管理员从已禁用空间上下文新增题目时不会提交到禁用空间", async () => {
  const user = userEvent.setup();
  const api = createQuestionAPI();
  const spacesApi = createSpaceAPI();
  renderWithFeedback(
    <MemoryRouter initialEntries={["/questions/new?space_id=303"]}>
      <Routes>
        <Route
          path="/questions/new"
          element={<QuestionCreatePage actorRole="tenant_admin" api={api} spaceApi={spacesApi} tenantID={10} />}
        />
        <Route path="/questions" element={<div>题库列表页</div>} />
      </Routes>
    </MemoryRouter>,
  );

  await waitFor(() => {
    expect(spacesApi.listSpaces).toHaveBeenCalledWith(expect.objectContaining({
      tenantID: 10,
      page: 1,
      pageSize: 100,
      filters: { status: "enabled" },
    }));
  });
  expect(screen.queryByRole("option", { name: "已禁用班级" })).not.toBeInTheDocument();
  expect(screen.queryByRole("option", { name: "当前空间题库" })).not.toBeInTheDocument();
  expect(screen.getByLabelText("所属空间")).toHaveValue("");

  fireEvent.change(screen.getByLabelText("题干"), { target: { value: "禁用空间上下文题目" } });
  fireEvent.change(screen.getByLabelText("选项 A"), { target: { value: "选项 A" } });
  fireEvent.change(screen.getByLabelText("选项 B"), { target: { value: "选项 B" } });
  fireEvent.change(screen.getByLabelText("题目解析"), { target: { value: "禁用空间上下文解析" } });
  await user.click(screen.getByRole("button", { name: "确认新增" }));

  expect(api.createQuestion).not.toHaveBeenCalled();
  expect(await screen.findByRole("alert")).toHaveTextContent("请选择有效的所属空间");
});

test("租户管理员从其他空间上下文新增题目后返回目标空间列表", async () => {
  const user = userEvent.setup();
  const api = createQuestionAPI();
  const spacesApi = createSpaceAPI();
  renderWithFeedback(
    <MemoryRouter initialEntries={["/questions/new?space_id=301"]}>
      <Routes>
        <Route
          path="/questions/new"
          element={<QuestionCreatePage actorRole="tenant_admin" api={api} spaceApi={spacesApi} tenantID={10} />}
        />
        <Route path="/questions" element={<LocationProbe />} />
      </Routes>
    </MemoryRouter>,
  );

  await waitFor(() => {
    expect(spacesApi.listSpaces).toHaveBeenCalledWith(expect.objectContaining({
      tenantID: 10,
      page: 1,
      pageSize: 100,
      filters: { status: "enabled" },
    }));
  });
  await user.selectOptions(screen.getByLabelText("所属空间"), "space:302");
  await user.type(screen.getByLabelText("题干"), "跨空间题目");
  await user.type(screen.getByLabelText("选项 A"), "选项 A");
  await user.type(screen.getByLabelText("选项 B"), "选项 B");
  await user.type(screen.getByLabelText("题目解析"), "跨空间解析");
  await user.click(screen.getByRole("button", { name: "确认新增" }));

  expect(await screen.findByText(/题库列表页/)).toHaveTextContent("?space_id=302");
});

function LocationProbe() {
  const location = useLocation();
  return <div>题库列表页 {location.search}</div>;
}

test("空间管理员在新增题目页面看不到公共题库选项", async () => {
  renderCreatePageWithRole("space_admin");

  expect(screen.queryByRole("option", { name: "公共题库" })).not.toBeInTheDocument();
  expect(screen.getByRole("option", { name: "当前空间题库" })).toBeInTheDocument();
});

test("新增选择题时不能把空白选项设为正确答案", async () => {
  const user = userEvent.setup();
  const api = renderCreatePage();

  await user.selectOptions(screen.getByLabelText("题型"), "multiple");
  await user.clear(screen.getByLabelText("题干"));
  await user.type(screen.getByLabelText("题干"), "选择所有正确选项");
  await user.clear(screen.getByLabelText("选项 A"));
  await user.type(screen.getByLabelText("选项 A"), "有效选项 A");
  await user.clear(screen.getByLabelText("选项 B"));
  await user.type(screen.getByLabelText("选项 B"), "有效选项 B");
  await user.click(screen.getByRole("button", { name: "添加选项" }));
  await user.clear(screen.getByLabelText("选项 C"));
  await user.click(screen.getByLabelText("设为正确答案 C"));
  await user.click(screen.getByLabelText("设为正确答案 A"));
  await user.click(screen.getByLabelText("设为正确答案 B"));
  await user.clear(screen.getByLabelText("题目解析"));
  await user.type(screen.getByLabelText("题目解析"), "空白选项不能作为正确答案。");
  await user.click(screen.getByRole("button", { name: "确认新增" }));

  const alert = await screen.findByRole("alert");
  expect(alert).toHaveTextContent("正确答案选项不能为空");
  expect(alert.closest(".ant-message")).toBeInTheDocument();
  expect(document.querySelector(".tenant-admin-warning")).not.toBeInTheDocument();
  expect(api.createQuestion).not.toHaveBeenCalled();
});

test("教师可以为填空题录入多个空的标准答案", async () => {
  const user = userEvent.setup();
  const api = renderCreatePage();

  await user.selectOptions(screen.getByLabelText("题型"), "fill_blank");
  await user.clear(screen.getByLabelText("题干"));
  await user.type(screen.getByLabelText("题干"), "Linux 常见目录 ____ 和 ____ 分别用于用户 home 与 root home。");
  await user.clear(screen.getByLabelText("第 1 空标准答案"));
  await user.type(screen.getByLabelText("第 1 空标准答案"), "/home");
  await user.click(screen.getByRole("button", { name: "新增填空答案" }));
  await user.type(screen.getByLabelText("第 2 空标准答案"), "/root");
  await user.clear(screen.getByLabelText("题目解析"));
  await user.type(screen.getByLabelText("题目解析"), "普通用户默认目录为 /home，root 用户目录为 /root。");
  await user.click(screen.getByRole("button", { name: "确认新增" }));

  expect(api.createQuestion).toHaveBeenCalledWith({
    tenantID: 10,
    spaceID: 301,
    type: "fill_blank",
    difficulty: "medium",
    title: "Linux 常见目录 ____ 和 ____ 分别用于用户 home 与 root home。",
    options: [],
    correctOptionIndexes: [],
    analysis: "普通用户默认目录为 /home，root 用户目录为 /root。",
    scoreDefault: "2",
    tags: [],
    standardAnswer: "[\"/home\",\"/root\"]",
    referenceAnswer: undefined,
    blankCount: 2,
  });
});

test("编辑题目加载失败时使用 toast 提示", async () => {
  const api = createQuestionAPI();
  api.getQuestion.mockRejectedValueOnce(new Error("load failed"));

  renderEditPage(api);

  const alert = await screen.findByRole("alert");
  expect(alert).toHaveTextContent("题目加载失败");
  expect(alert.closest(".ant-message")).toBeInTheDocument();
  expect(document.querySelector(".tenant-admin-warning")).not.toBeInTheDocument();
});

test("教师可以打开编辑题目页面并保存修改", async () => {
  const user = userEvent.setup();
  const api = renderEditPage();

  expect(await screen.findByRole("heading", { name: "编辑题目" })).toBeInTheDocument();
  expect(api.getQuestion).toHaveBeenCalledWith({ tenantID: 10, questionID: 100 });
  expect(screen.getByLabelText("题干")).toHaveValue("原题干");
  await user.clear(screen.getByLabelText("题干"));
  await user.type(screen.getByLabelText("题干"), "更新后题干");
  await user.clear(screen.getByLabelText("题目解析"));
  await user.type(screen.getByLabelText("题目解析"), "更新后解析");
  await user.click(screen.getByRole("button", { name: "确认保存" }));

  expect(api.updateQuestion).toHaveBeenCalledWith({
    tenantID: 10,
    spaceID: 301,
    questionID: 100,
    type: "single",
    difficulty: "easy",
    title: "更新后题干",
    options: ["原选项 A", "原选项 B"],
    correctOptionIndexes: [0],
    analysis: "更新后解析",
    scoreDefault: "4",
    tags: ["阅读理解"],
    standardAnswer: undefined,
    referenceAnswer: undefined,
    blankCount: undefined,
  });
  expect(await screen.findByText("题库列表页")).toBeInTheDocument();
});

test("编辑简答题时参考答案使用 Markdown 编辑器并保存原文", async () => {
  const user = userEvent.setup();
  const api = createQuestionAPI();
  api.getQuestion.mockResolvedValueOnce({
    id: 100,
    tenantID: 10,
    spaceID: 301,
    type: "short_text",
    title: "请简述闭区间上连续函数的性质。",
    stem: "请简述闭区间上连续函数的性质。",
    options: [],
    correctOptionIndexes: [],
    referenceAnswer: "## 原参考\n- 有界性\n- 最值定理",
    analysis: "围绕闭区间和连续两个条件说明。",
    difficulty: "easy",
    tag: "函数",
    tags: ["函数"],
    scoreDefault: "6",
    authorName: "teacher01",
    authorRole: "teacher",
    createdAt: 1700000000000,
    status: "ready",
  });

  renderEditPage(api);

  expect(await screen.findByLabelText("简答题参考答案")).toHaveValue("## 原参考\n- 有界性\n- 最值定理");
  const referenceEditor = screen.getByLabelText("简答题参考答案").closest(".markdown-editor");
  expect(referenceEditor).not.toBeNull();
  expect(referenceEditor?.querySelector(".w-md-editor")).toBeInTheDocument();
  expect(within(referenceEditor as HTMLElement).getByRole("heading", { name: "原参考" })).toBeInTheDocument();

  const nextReferenceAnswer = "## 新参考\n- 闭区间连续函数有界\n- 可以取得最大值和最小值";
  fireEvent.change(screen.getByLabelText("简答题参考答案"), { target: { value: nextReferenceAnswer } });
  await user.click(screen.getByRole("button", { name: "确认保存" }));

  expect(api.updateQuestion).toHaveBeenCalledWith(expect.objectContaining({
    questionID: 100,
    type: "short_text",
    referenceAnswer: nextReferenceAnswer,
    standardAnswer: undefined,
  }));
});

test("编辑题目页面必填项使用红色星号标记", async () => {
  renderEditPage();

  expect(await screen.findByRole("heading", { name: "编辑题目" })).toBeInTheDocument();
  expect(within(screen.getByLabelText("选择题选项")).getByText("支持公式：行内 $a^2+b^2=c^2$，块级 $$\\sum_{i=1}^{n} i$$")).toHaveClass("exam-option-list__formula-hint");

  for (const label of ["题型", "题目难度", "默认分值", "题干", "题目解析", "选项 A", "选项 B"]) {
    const field = screen.getByLabelText(label).closest(".field");
    expect(field).not.toBeNull();
    expect(within(field as HTMLElement).getByText("*")).toHaveClass("required-marker");
  }
});

test("编辑填空题时会加载多个空答案", async () => {
  const user = userEvent.setup();
  const api = createQuestionAPI();
  api.getQuestion.mockResolvedValueOnce({
    id: 100,
    tenantID: 10,
    type: "fill_blank",
    title: "请填写两个目录",
    stem: "请填写两个目录",
    options: [],
    analysis: "解析",
    difficulty: "easy",
    tag: "系统",
    tags: ["系统"],
    scoreDefault: "4",
    standardAnswer: "[\"/etc\",\"/var\"]",
    blankAnswers: ["/etc", "/var"],
    authorName: "teacher01",
    authorRole: "teacher",
    createdAt: 1700000000000,
    status: "ready",
  });

  renderEditPage(api);

  expect(await screen.findByLabelText("第 1 空标准答案")).toHaveValue("/etc");
  expect(screen.getByLabelText("第 2 空标准答案")).toHaveValue("/var");
  await user.clear(screen.getByLabelText("第 2 空标准答案"));
  await user.type(screen.getByLabelText("第 2 空标准答案"), "/usr");
  await user.click(screen.getByRole("button", { name: "确认保存" }));

  expect(api.updateQuestion).toHaveBeenCalledWith(expect.objectContaining({
    questionID: 100,
    type: "fill_blank",
    standardAnswer: "[\"/etc\",\"/usr\"]",
    blankCount: 2,
  }));
});
