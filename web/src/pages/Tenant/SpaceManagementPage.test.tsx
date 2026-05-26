import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import { SpaceManagementPage } from "./SpaceManagementPage";

function createSpaceAPI() {
  return {
    listSpaces: vi.fn().mockResolvedValue({
      items: [
        {
          id: 100,
          tenantID: 10,
          name: "高一 1 班",
          description: "高一语文月考主空间",
          logoFileName: "class1.png",
          members: [
            { id: 1, name: "李老师", role: "space_admin", status: "enabled" },
            { id: 2, name: "张同学", role: "student", status: "enabled" },
          ],
        },
        {
          id: 101,
          tenantID: 10,
          name: "高一 2 班",
          description: "阶段测评与补测空间",
          logoFileName: "class2.png",
          members: [{ id: 3, name: "周老师", role: "space_admin", status: "enabled" }],
        },
      ],
    }),
    createSpace: vi.fn().mockResolvedValue({
      id: 102,
      tenantID: 10,
      name: "高一 3 班",
      description: "面向月考与补测的学生空间",
      logoFileName: "class3.png",
      members: [{ id: 4, name: "赵老师", role: "space_admin", status: "enabled" }],
    }),
  };
}

test("空间管理页展示空间列表和空间管理员", async () => {
  const api = createSpaceAPI();
  render(<SpaceManagementPage api={api} tenantID={10} />);

  expect(screen.getAllByRole("tab")).toHaveLength(1);
  expect(screen.getByRole("tab", { name: "空间管理" })).toHaveAttribute("aria-selected", "true");
  expect(screen.queryByRole("heading", { name: "空间管理" })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "创建空间" })).toHaveClass("tenant-create-button");
  expect(screen.getByRole("button", { name: "保存空间配置" })).toBeInTheDocument();
  expect(screen.getByLabelText("搜索空间")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "刷新空间列表" })).toBeInTheDocument();
  expect(await screen.findByText("高一 1 班")).toBeInTheDocument();
  expect(screen.getByText("空间管理员：李老师")).toBeInTheDocument();
  expect(api.listSpaces).toHaveBeenCalledWith(10);
  expect(screen.queryByLabelText("成员姓名")).not.toBeInTheDocument();
});

test("租户管理员可以创建空间并上传 logo", async () => {
  const user = userEvent.setup();
  const api = createSpaceAPI();
  render(<SpaceManagementPage api={api} tenantID={10} />);

  await screen.findByText("高一 1 班");

  await user.click(screen.getByRole("button", { name: "创建空间" }));

  expect(screen.getByRole("dialog", { name: "创建空间弹窗" })).toBeInTheDocument();

  await user.type(screen.getByLabelText("空间名称"), "高一 3 班");
  await user.type(screen.getByLabelText("空间描述"), "面向月考与补测的学生空间");
  await user.type(screen.getByLabelText("空间管理员"), "赵老师");
  await user.upload(screen.getByLabelText("空间 Logo"), new File(["logo"], "class3.png", { type: "image/png" }));
  await user.click(screen.getByRole("button", { name: "确认创建" }));

  expect(api.createSpace).toHaveBeenCalledWith({
    tenantID: 10,
    name: "高一 3 班",
    description: "面向月考与补测的学生空间",
    logoFileName: "class3.png",
    adminName: "赵老师",
  });
  expect(await screen.findByText("高一 3 班")).toBeInTheDocument();
  expect(screen.getByText("空间管理员：赵老师")).toBeInTheDocument();
  expect(screen.getByText("class3.png")).toBeInTheDocument();
});

test("租户管理员可以搜索和刷新空间列表", async () => {
  const user = userEvent.setup();
  render(<SpaceManagementPage api={createSpaceAPI()} tenantID={10} />);

  await screen.findByText("高一 1 班");

  await user.type(screen.getByLabelText("搜索空间"), "高一 2 班");
  await user.click(screen.getByRole("button", { name: "搜索" }));

  expect(screen.queryByText("高一 1 班")).not.toBeInTheDocument();
  expect(screen.getByText("高一 2 班")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "刷新空间列表" }));

  expect(screen.getByText("高一 1 班")).toBeInTheDocument();
  expect(screen.getByText("高一 2 班")).toBeInTheDocument();
});

test("租户管理员可以编辑空间描述并管理成员", async () => {
  const user = userEvent.setup();
  render(<SpaceManagementPage api={createSpaceAPI()} tenantID={10} />);

  await screen.findByText("高一 1 班");

  await user.click(within(screen.getByRole("row", { name: /高一 1 班/ })).getByRole("button", { name: "编辑描述" }));
  await user.clear(screen.getByLabelText("编辑空间描述"));
  await user.type(screen.getByLabelText("编辑空间描述"), "高一语文月考主空间");
  await user.click(screen.getByRole("button", { name: "保存空间描述" }));

  expect(screen.getByText("高一语文月考主空间")).toBeInTheDocument();

  await user.click(within(screen.getByRole("row", { name: /高一 1 班/ })).getByRole("button", { name: "成员管理" }));

  expect(screen.getByText("高一 1 班成员")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "返回空间列表" })).toBeInTheDocument();

  await user.type(screen.getByLabelText("成员姓名"), "陈同学");
  await user.selectOptions(screen.getByLabelText("成员角色"), "student");
  await user.click(screen.getByRole("button", { name: "添加成员" }));

  expect(screen.getByText("陈同学")).toBeInTheDocument();
  expect(screen.getAllByText("学生").length).toBeGreaterThan(0);
});

test("降级最后一个空间管理员时展示不变式错误", async () => {
  const user = userEvent.setup();
  render(<SpaceManagementPage api={createSpaceAPI()} tenantID={10} />);

  await screen.findByText("高一 1 班");

  await user.click(within(screen.getByRole("row", { name: /高一 1 班/ })).getByRole("button", { name: "成员管理" }));
  await user.click(within(screen.getByRole("row", { name: /李老师/ })).getByRole("button", { name: "降级为教师" }));

  expect(screen.getByRole("alert")).toHaveTextContent("空间至少保留一个启用状态的空间管理员");
});

test("成员管理只作用于从空间列表选中的空间", async () => {
  const user = userEvent.setup();
  render(<SpaceManagementPage api={createSpaceAPI()} tenantID={10} />);

  await screen.findByText("高一 1 班");

  await user.click(within(screen.getByRole("row", { name: /高一 2 班/ })).getByRole("button", { name: "成员管理" }));

  expect(screen.getByText("高一 2 班成员")).toBeInTheDocument();
  expect(screen.getByText("周老师")).toBeInTheDocument();
  expect(screen.queryByText("李老师")).not.toBeInTheDocument();

  await user.type(screen.getByLabelText("成员姓名"), "陈同学");
  await user.selectOptions(screen.getByLabelText("成员角色"), "student");
  await user.click(screen.getByRole("button", { name: "添加成员" }));

  expect(screen.getByText("陈同学")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "返回空间列表" }));
  await user.click(within(screen.getByRole("row", { name: /高一 1 班/ })).getByRole("button", { name: "成员管理" }));

  expect(screen.getByText("高一 1 班成员")).toBeInTheDocument();
  expect(screen.queryByText("陈同学")).not.toBeInTheDocument();
});

test("空间配置保存后展示当前配置", async () => {
  const user = userEvent.setup();
  render(<SpaceManagementPage api={createSpaceAPI()} tenantID={10} />);

  await screen.findByText("高一 1 班");

  await user.click(screen.getByRole("button", { name: "保存空间配置" }));

  expect(screen.getByRole("dialog", { name: "空间配置弹窗" })).toBeInTheDocument();

  await user.clear(screen.getByLabelText("默认考试时长"));
  await user.type(screen.getByLabelText("默认考试时长"), "90");
  await user.click(screen.getByLabelText("允许学生查看练习解析"));
  await user.click(screen.getByRole("button", { name: "确认保存配置" }));

  expect(screen.queryByRole("dialog", { name: "空间配置弹窗" })).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "保存空间配置" }));

  expect(screen.getByLabelText("默认考试时长")).toHaveValue(90);
  expect(screen.getByLabelText("允许学生查看练习解析")).not.toBeChecked();
});
