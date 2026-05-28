import { Save } from "lucide-react";
import { useEffect, useState } from "react";
import { formatApiErrorMessage } from "../../api/client";
import { profileApi } from "../../api/profile";
import type { Profile, ProfileAPI } from "../../api/profile";
import { useSession } from "../../auth/session-context";
import { Button } from "../../components/ui/Button";
import { Panel } from "../../components/ui/Panel";
import { StatusBadge } from "../../components/ui/StatusBadge";

type ProfileSettingsPageProps = {
  api?: ProfileAPI;
};

export function ProfileSettingsPage({ api = profileApi }: ProfileSettingsPageProps) {
  const { session, signIn } = useSession();
  const [profile, setProfile] = useState<Profile | null>(null);
  const [displayName, setDisplayName] = useState("");
  const [avatarURL, setAvatarURL] = useState("");
  const [phone, setPhone] = useState("");
  const [email, setEmail] = useState("");
  const [loadError, setLoadError] = useState("");
  const [saveMessage, setSaveMessage] = useState("");
  const [isSaving, setIsSaving] = useState(false);
  const isPlatformUser = profile?.subjectType === "platform_user";

  useEffect(() => {
    let ignore = false;
    api.getProfile()
      .then((nextProfile) => {
        if (ignore) {
          return;
        }
        setProfile(nextProfile);
        setDisplayName(nextProfile.displayName);
        setAvatarURL(nextProfile.avatarURL);
        setPhone(nextProfile.phone);
        setEmail(nextProfile.email);
        setLoadError("");
      })
      .catch((err: unknown) => {
        if (!ignore) {
          setLoadError(formatApiErrorMessage(err, "个人资料加载失败"));
        }
      });
    return () => {
      ignore = true;
    };
  }, [api]);

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsSaving(true);
    setSaveMessage("");
    try {
      // 个人资料保存成功后同步本地 session，侧边栏和后续页面能立即使用新显示名称。
      const nextProfile = await api.updateProfile({
        displayName: displayName.trim(),
        avatarURL: avatarURL.trim(),
        phone: phone.trim(),
        email: email.trim(),
      });
      setProfile(nextProfile);
      setDisplayName(nextProfile.displayName);
      setAvatarURL(nextProfile.avatarURL);
      setPhone(nextProfile.phone);
      setEmail(nextProfile.email);
      if (session) {
        signIn({
          ...session,
          user: {
            ...session.user,
            displayName: nextProfile.displayName,
            tenantID: nextProfile.tenantID ?? session.user.tenantID,
          },
        });
      }
      setSaveMessage("个人资料已保存");
    } catch (err) {
      setSaveMessage(formatApiErrorMessage(err, "保存个人资料失败"));
    } finally {
      setIsSaving(false);
    }
  }

  return (
    <section className="page platform-page profile-settings-page">
      <nav aria-label="个人设置菜单" className="platform-tabbar" role="tablist">
        <span className="platform-tab platform-tab--active" role="tab" aria-selected="true">
          个人设置
        </span>
      </nav>

      <div className="toolbar">
        <div>
          <h1>个人设置</h1>
          <p>维护当前登录账号的显示名称、头像、手机号和邮箱。</p>
        </div>
        {profile && <StatusBadge tone="info">{roleLabel(profile.role)}</StatusBadge>}
      </div>

      <div className="profile-settings-grid">
        <Panel className="profile-account-card" title="账号信息" subtitle="这些信息来自当前登录态对应的账号。">
          <div className="profile-account-grid">
            <div className="profile-info-row">
              <strong>用户 ID</strong>
              <span>{profile?.userID ?? "-"}</span>
            </div>
            <div className="profile-info-row">
              <strong>租户 ID</strong>
              <span>{profile?.tenantID ?? "-"}</span>
            </div>
            <div className="profile-info-row profile-info-row--wide">
              <strong>身份类型</strong>
              <span>{profile?.subjectType === "platform_user" ? "平台管理员" : "租户用户"}</span>
            </div>
          </div>
        </Panel>

        <Panel className="profile-form-card" title="基础信息" subtitle="保存后会立即更新本地登录态显示名称。">
          <form className="profile-form" onSubmit={handleSubmit}>
            {loadError && <p className="profile-warning">{loadError}</p>}
            <label className="profile-field">
              <span>{isPlatformUser ? "登录账号" : "显示名称"}</span>
              <input
                aria-label={isPlatformUser ? "登录账号" : "显示名称"}
                disabled={isPlatformUser}
                onChange={(event) => setDisplayName(event.target.value)}
                placeholder={isPlatformUser ? "平台管理员登录账号" : "请输入显示名称"}
                required
                value={displayName}
              />
            </label>
            <label className="profile-field">
              <span>头像地址</span>
              <input
                aria-label="头像地址"
                onChange={(event) => setAvatarURL(event.target.value)}
                placeholder="/uploads/avatars/me.png"
                value={avatarURL}
              />
            </label>
            <label className="profile-field">
              <span>手机号</span>
              <input
                aria-label="手机号"
                onChange={(event) => setPhone(event.target.value)}
                placeholder="请输入手机号"
                value={phone}
              />
            </label>
            <label className="profile-field">
              <span>邮箱</span>
              <input
                aria-label="邮箱"
                onChange={(event) => setEmail(event.target.value)}
                placeholder="name@example.com"
                type="email"
                value={email}
              />
            </label>
            <Button className="profile-save-button" disabled={isSaving} variant="primary" type="submit">
              <Save aria-hidden="true" size={16} />
              保存资料
            </Button>
            {saveMessage && (
              <p aria-label="profile-save-result" className="profile-help" role="status">
                {saveMessage}
              </p>
            )}
          </form>
        </Panel>
      </div>
    </section>
  );
}

function roleLabel(role: string) {
  switch (role) {
    case "platform_admin":
      return "平台管理员";
    case "tenant_admin":
      return "租户管理员";
    case "teacher":
      return "教师";
    case "student":
      return "学生";
    default:
      return role;
  }
}
