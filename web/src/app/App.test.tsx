import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, vi } from "vitest";
import { MemoryRouter } from "react-router-dom";
import { App } from "./App";
import { AppProviders } from "./providers";

const originalMatchMedia = window.matchMedia;

afterEach(() => {
  Object.defineProperty(window, "matchMedia", {
    value: originalMatchMedia,
    writable: true,
  });
});

function mockExamViewport(matchesNarrow: boolean) {
  Object.defineProperty(window, "matchMedia", {
    value: vi.fn().mockImplementation((query: string) => ({
      matches: query === "(max-width: 1100px)" ? matchesNarrow : false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
    writable: true,
  });
}

function renderApp(initialEntries: string[]) {
  return render(
    <AppProviders>
      <MemoryRouter initialEntries={initialEntries}>
        <App />
      </MemoryRouter>
    </AppProviders>,
  );
}

test("渲染 Papermind 管理端基础骨架", () => {
  renderApp(["/"]);

  expect(screen.getByText("Papermind")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: /租户管理/ })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: /阅卷中心/ })).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "考试平台概览" })).toBeInTheDocument();
});

test("学生考试端与管理员后台路由隔离", () => {
  renderApp(["/student/exam"]);

  expect(screen.getByRole("heading", { name: "期中考试（高一语文）" })).toBeInTheDocument();
  expect(screen.getByText("PaperMind")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "张三" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "退出考试" })).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "答题卡" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "交 卷" })).toBeInTheDocument();
  expect(screen.queryByText("考生编号：S1001001")).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "计算器" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "草稿纸" })).not.toBeInTheDocument();
  expect(screen.queryByLabelText("后台导航")).not.toBeInTheDocument();
  expect(screen.queryByRole("link", { name: /租户管理/ })).not.toBeInTheDocument();
});

test("考生编号折叠在右侧用户菜单中", async () => {
  const user = userEvent.setup();

  renderApp(["/student/exam"]);

  const profileButton = screen.getByRole("button", { name: "张三" });
  expect(profileButton).toHaveAttribute("aria-expanded", "false");
  expect(screen.queryByRole("menu", { name: "考生信息" })).not.toBeInTheDocument();

  await user.click(profileButton);

  expect(profileButton).toHaveAttribute("aria-expanded", "true");
  expect(screen.getByRole("menu", { name: "考生信息" })).toHaveTextContent("考生编号：S1001001");

  await user.click(screen.getByRole("button", { name: "收起题目列表" }));

  expect(profileButton).toHaveAttribute("aria-expanded", "false");
  expect(screen.queryByRole("menu", { name: "考生信息" })).not.toBeInTheDocument();
});

test("学生考试端窄屏辅助信息使用抽屉展开", async () => {
  const user = userEvent.setup();

  renderApp(["/student/exam"]);

  const drawerToggle = screen.getByRole("button", { name: "考试信息与答题卡" });
  expect(drawerToggle).toHaveAttribute("aria-expanded", "false");

  await user.click(drawerToggle);

  expect(drawerToggle).toHaveAttribute("aria-expanded", "true");
  expect(screen.getByLabelText("考试辅助抽屉")).toHaveClass("exam-right-column--open");

  await user.click(screen.getByRole("button", { name: "关闭考试抽屉" }));

  expect(drawerToggle).toHaveAttribute("aria-expanded", "false");
});

test("H5 开考前页面进入移动端作答流程", async () => {
  const user = userEvent.setup();
  mockExamViewport(true);

  renderApp(["/student/exam"]);

  expect(screen.getByLabelText("开考前说明")).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "期中考试（高一语文）" })).toBeInTheDocument();
  expect(screen.getByText("开考前")).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "考试须知" })).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "开始考试" }));

  expect(screen.getByLabelText("移动端单选题作答")).toBeInTheDocument();
  expect(screen.getByText((_content, node) => node?.textContent === "1 / 45")).toBeInTheDocument();
  expect(screen.getByText("考试中请勿切换页面或离开考试")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "答题卡" })).toBeInTheDocument();
});

test("H5 答题卡通过底部入口展示并弹出确认交卷", async () => {
  const user = userEvent.setup();
  mockExamViewport(true);

  renderApp(["/student/exam"]);

  await user.click(screen.getByRole("button", { name: "开始考试" }));
  await user.click(screen.getByRole("button", { name: "答题卡" }));

  expect(screen.getByLabelText("移动端答题卡")).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "答题卡" })).toBeInTheDocument();
  expect(within(screen.getByLabelText("答题统计")).getByText("28")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "提交试卷" })).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "提交试卷" }));

  expect(screen.getByRole("dialog", { name: "确认交卷" })).toBeInTheDocument();
  expect(screen.getByText("交卷后将无法继续作答，请确认是否提交？")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "取消" }));

  expect(screen.queryByRole("dialog", { name: "确认交卷" })).not.toBeInTheDocument();
});

test("确认交卷通过交互弹窗展示", async () => {
  const user = userEvent.setup();

  renderApp(["/student/exam"]);

  expect(screen.queryByRole("dialog", { name: "确认交卷" })).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "交 卷" }));

  expect(screen.getByRole("dialog", { name: "确认交卷" })).toBeInTheDocument();
  expect(screen.getByText("交卷后将无法继续作答，请确认是否交卷？")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "取消" }));

  expect(screen.queryByRole("dialog", { name: "确认交卷" })).not.toBeInTheDocument();
});

test("自动保存提示不作为页面正文静态展示", () => {
  renderApp(["/student/exam"]);

  expect(screen.queryByRole("status", { name: "自动保存提示" })).not.toBeInTheDocument();
  expect(screen.queryByText("答案已自动保存")).not.toBeInTheDocument();
});

test("简答题作答效果不作为当前考试页面正文展示", () => {
  renderApp(["/student/exam"]);

  expect(screen.queryByText("四、简答题（共30分）")).not.toBeInTheDocument();
  expect(screen.queryByPlaceholderText("请输入作答内容...")).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "保存" })).not.toBeInTheDocument();
});

test("离开页面提醒通过确认弹窗展示", async () => {
  const user = userEvent.setup();

  renderApp(["/student/exam"]);

  expect(screen.queryByRole("dialog", { name: "离开页面提醒" })).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "退出考试" }));

  expect(screen.getByRole("dialog", { name: "离开页面提醒" })).toBeInTheDocument();
  expect(screen.getByText("检测到您将离开考试页面，请确认是否离开？")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "留在页面" }));

  expect(screen.queryByRole("dialog", { name: "离开页面提醒" })).not.toBeInTheDocument();
});
