import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { expect, test, vi } from "vitest";
import { QuestionCreatePage } from "./QuestionCreatePage";

function createQuestionAPI() {
  return {
    listQuestions: vi.fn().mockResolvedValue({
      items: [
        {
          id: 100,
          tenantID: 10,
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
      status: "ready",
    }),
    getQuestion: vi.fn().mockResolvedValue({
      id: 100,
      tenantID: 10,
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
  };
}

function renderCreatePage(api = createQuestionAPI()) {
  render(
    <MemoryRouter initialEntries={["/questions/new?space_id=301"]}>
      <Routes>
        <Route path="/questions/new" element={<QuestionCreatePage api={api} tenantID={10} spaceID={301} />} />
        <Route path="/questions" element={<div>题库列表页</div>} />
      </Routes>
    </MemoryRouter>,
  );
  return api;
}

function renderEditPage(api = createQuestionAPI()) {
  render(
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

  await user.type(screen.getByLabelText("题干"), "## 函数题\n**单调递增**");
  await user.type(screen.getByLabelText("题目解析"), "- 看斜率\n- 排除反例");

  const stemPreview = within(previewPanels[0] as HTMLElement);
  const analysisPreview = within(previewPanels[1] as HTMLElement);
  expect(stemPreview.getByRole("heading", { name: "函数题" })).toBeInTheDocument();
  expect(stemPreview.getByText("单调递增")).toBeInTheDocument();
  expect(analysisPreview.getByText("看斜率")).toBeInTheDocument();
  expect(analysisPreview.getByText("排除反例")).toBeInTheDocument();
});

test("教师可以在新增题目页面创建选择题并编辑选项和标签", async () => {
  const user = userEvent.setup();
  const api = renderCreatePage();

  await user.selectOptions(screen.getByLabelText("题型"), "multiple");
  await user.selectOptions(screen.getByLabelText("题目难度"), "hard");
  await user.clear(screen.getByLabelText("默认分值"));
  await user.type(screen.getByLabelText("默认分值"), "6");
  await user.clear(screen.getByLabelText("题干"));
  await user.type(screen.getByLabelText("题干"), "## 函数题\n下列函数在 R 上**单调递增**的是哪一项？");
  await user.clear(screen.getByLabelText("选项 A"));
  await user.type(screen.getByLabelText("选项 A"), "y = x");
  await user.clear(screen.getByLabelText("选项 B"));
  await user.type(screen.getByLabelText("选项 B"), "y = -x");
  await user.click(screen.getByRole("button", { name: "添加选项" }));
  await user.clear(screen.getByLabelText("选项 C"));
  await user.type(screen.getByLabelText("选项 C"), "y = x + 1");
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
    options: ["y = x", "y = -x", "y = x + 1"],
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

  expect(await screen.findByRole("alert")).toHaveTextContent("正确答案选项不能为空");
  expect(api.createQuestion).not.toHaveBeenCalled();
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
