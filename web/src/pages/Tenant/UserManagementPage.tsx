import { Button } from "../../components/ui/Button";
import { RefreshCw, Search } from "lucide-react";
import { useState } from "react";
import { FileUploadField } from "../../components/ui/FileUploadField";
import { Panel } from "../../components/ui/Panel";
import { StatusBadge } from "../../components/ui/StatusBadge";

type UserRole = "tenant_admin" | "teacher" | "student";

type TenantUser = {
  id: number;
  name: string;
  username: string;
  role: UserRole;
  avatarFileName: string;
  status: "enabled" | "disabled";
};

const roleLabels: Record<UserRole, string> = {
  tenant_admin: "租户管理员",
  teacher: "教师",
  student: "学生",
};

const initialUsers: TenantUser[] = [
  {
    id: 1,
    name: "李老师",
    username: "li.teacher",
    role: "teacher",
    avatarFileName: "li.png",
    status: "enabled",
  },
  {
    id: 2,
    name: "张同学",
    username: "zhang.student",
    role: "student",
    avatarFileName: "zhang.png",
    status: "disabled",
  },
];

export function UserManagementPage() {
  const [users, setUsers] = useState<TenantUser[]>(initialUsers);
  const [name, setName] = useState("");
  const [username, setUsername] = useState("");
  const [role, setRole] = useState<UserRole>("student");
  const [avatarFileName, setAvatarFileName] = useState("");
  const [avatarResetKey, setAvatarResetKey] = useState(0);
  const [importFileName, setImportFileName] = useState("");
  const [importMessage, setImportMessage] = useState("");
  const [disableTarget, setDisableTarget] = useState<TenantUser | null>(null);
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);
  const [isImportDialogOpen, setIsImportDialogOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [appliedSearchQuery, setAppliedSearchQuery] = useState("");

  const filteredUsers = users.filter((item) => {
    const keyword = appliedSearchQuery.trim().toLowerCase();
    if (!keyword) {
      return true;
    }

    // 用户列表搜索只匹配当前表格可见字段，便于租户管理员按姓名、账号、角色或状态定位。
    return [
      item.name,
      item.username,
      item.avatarFileName,
      roleLabels[item.role],
      item.status === "enabled" ? "启用" : "禁用",
    ].some((value) => value.toLowerCase().includes(keyword));
  });

  function handleCreateUser(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    // 租户管理员创建用户时先写入基础身份和角色，空间归属后续在空间成员管理中维护。
    setUsers((items) => [
      ...items,
      {
        id: Date.now(),
        name,
        username,
        role,
        avatarFileName: avatarFileName || "未上传",
        status: "enabled",
      },
    ]);
    setName("");
    setUsername("");
    setRole("student");
    setAvatarFileName("");
    setAvatarResetKey((value) => value + 1);
    setIsCreateDialogOpen(false);
  }

  function handleImportUsers() {
    if (!importFileName) {
      setImportMessage("请选择用户导入文件");
      return;
    }

    // 导入结果先以摘要展示，真实行级错误会在接入导入 API 后由服务端返回。
    setImportMessage(`${importFileName} 已解析 24 个用户`);
    setIsImportDialogOpen(false);
  }

  function confirmDisableUser() {
    if (!disableTarget) {
      return;
    }

    setUsers((items) =>
      items.map((item) =>
        item.id === disableTarget.id ? { ...item, status: "disabled" } : item,
      ),
    );
    setDisableTarget(null);
  }

  function handleSearchUsers() {
    setAppliedSearchQuery(searchQuery);
  }

  function handleRefreshUsers() {
    setSearchQuery("");
    setAppliedSearchQuery("");
  }

  return (
    <section className="page platform-page tenant-admin-page">
      <nav aria-label="用户管理菜单" className="platform-tabbar" role="tablist">
        <a className="platform-tab platform-tab--active" href="/users" role="tab" aria-selected="true">
          用户管理
        </a>
      </nav>

      <Panel>
          <div className="tenant-list-toolbar">
            <div className="tenant-list-actions" aria-label="用户操作区">
              <Button
              variant="toolbarPrimary"
              onClick={() => setIsCreateDialogOpen(true)}
              type="button"
            >
              创建用户
            </Button>
            <Button
              variant="toolbarSecondary"
              onClick={() => setIsImportDialogOpen(true)}
              type="button"
            >
                导入用户
              </Button>
            </div>
            <div className="tenant-search-actions">
              <label className="tenant-search-field">
                <span className="sr-only">搜索用户</span>
                <input
                  onChange={(event) => setSearchQuery(event.target.value)}
                  placeholder="输入姓名、账号、角色或状态"
                  value={searchQuery}
                />
              </label>
              <Button aria-label="搜索" variant="icon" onClick={handleSearchUsers} type="button">
                <Search aria-hidden="true" size={16} />
              </Button>
              <Button
                aria-label="刷新用户列表"
                variant="icon"
                onClick={handleRefreshUsers}
                type="button"
              >
                <RefreshCw aria-hidden="true" size={16} />
              </Button>
            </div>
          </div>
        {importMessage && <div className="tenant-admin-status" role="status">{importMessage}</div>}
        <div className="table-wrap">
          <table className="data-table tenant-admin-table">
            <thead>
              <tr>
                <th scope="col">用户</th>
                <th scope="col">账号</th>
                <th scope="col">头像</th>
                <th scope="col">角色</th>
                <th scope="col">状态</th>
                <th scope="col">操作</th>
              </tr>
            </thead>
            <tbody>
              {filteredUsers.map((item) => (
                <tr key={item.id}>
                  <td>{item.name}</td>
                  <td>{item.username}</td>
                  <td>{item.avatarFileName}</td>
                  <td>{roleLabels[item.role]}</td>
                  <td>
                    <StatusBadge tone={item.status === "enabled" ? "success" : "info"}>
                      {item.status === "enabled" ? "启用" : "禁用"}
                    </StatusBadge>
                  </td>
                  <td>
                    <Button
                      variant="actionClose"
                      disabled={item.status === "disabled"}
                      onClick={() => setDisableTarget(item)}
                      type="button"
                    >
                      禁用用户
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Panel>

      {isCreateDialogOpen && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="创建用户弹窗">
          <div className="platform-dialog__card">
            <h2>创建用户</h2>
            <form className="platform-form" onSubmit={handleCreateUser}>
              <label className="field">
                <span>姓名</span>
                <input onChange={(event) => setName(event.target.value)} required value={name} />
              </label>
              <label className="field">
                <span>账号</span>
                <input onChange={(event) => setUsername(event.target.value)} required value={username} />
              </label>
              <label className="field">
                <span>角色</span>
                <select onChange={(event) => setRole(event.target.value as UserRole)} value={role}>
                  <option value="tenant_admin">租户管理员角色</option>
                  <option value="teacher">教师角色</option>
                  <option value="student">学生角色</option>
                </select>
              </label>
              <FileUploadField
                accept={["image/png", "image/jpeg"]}
                key={avatarResetKey}
                label="用户头像"
                maxSizeBytes={1024 * 1024}
                onFileAccepted={(file) => setAvatarFileName(file.name)}
              />
              <div className="platform-dialog__actions">
                <Button variant="secondary" onClick={() => setIsCreateDialogOpen(false)} type="button">
                  取消
                </Button>
                <Button variant="primary" type="submit">
                  确认创建
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}

      {isImportDialogOpen && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="导入用户弹窗">
          <div className="platform-dialog__card">
            <h2>导入用户</h2>
            <div className="platform-form">
              <FileUploadField
                accept={["text/csv", "application/vnd.ms-excel"]}
                label="用户导入文件"
                maxSizeBytes={2 * 1024 * 1024}
                onFileAccepted={(file) => setImportFileName(file.name)}
              />
              <div className="platform-dialog__actions">
                <Button variant="secondary" onClick={() => setIsImportDialogOpen(false)} type="button">
                  取消
                </Button>
                <Button variant="primary" onClick={handleImportUsers} type="button">
                  确认导入
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}

      {disableTarget && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="禁用用户提示">
          <div className="platform-dialog__card">
            <h2>禁用 {disableTarget.name}</h2>
            <p className="tenant-admin-warning">
              将失去登录能力，阅卷和考试发布权限会被收回，已提交答卷和历史成绩不受影响。
            </p>
            <div className="platform-dialog__actions">
              <Button variant="secondary" onClick={() => setDisableTarget(null)} type="button">
                取消
              </Button>
              <Button variant="primary" onClick={confirmDisableUser} type="button">
                确认禁用
              </Button>
            </div>
          </div>
        </div>
      )}
    </section>
  );
}
