import { Avatar } from "@/components/Avatar";
import { IceFlowBackground } from "@/components/IceFlowBackground";
import { useDemoStore } from "@/store/DemoStore";
import {
  Bell,
  BellRing,
  Check,
  LogOut,
  Menu,
  RotateCcw,
  UserRound,
  X,
} from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Link, NavLink, Outlet, useLocation } from "react-router-dom";

const navigation = [
  { to: "/explore", label: "探索技能" },
  { to: "/creator", label: "创作者中心" },
  { to: "/trust", label: "可信评测" },
];

export default function AppShell() {
  const { state, dispatch } = useDemoStore();
  const location = useLocation();
  const loginDialogRef = useRef<HTMLDialogElement>(null);
  const [notificationsOpen, setNotificationsOpen] = useState(false);
  const [accountOpen, setAccountOpen] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const isHome = location.pathname === "/";
  const currentUser = state.users.find((user) => user.id === state.currentUserId)!;
  const unread = state.social.notifications.filter((item) => !item.read).length;

  useEffect(() => {
    setMobileOpen(false);
    setNotificationsOpen(false);
    setAccountOpen(false);
    window.scrollTo({ top: 0, behavior: "instant" });
  }, [location.pathname]);

  useEffect(() => {
    document.documentElement.classList.toggle("is-home-route", isHome);
    return () => document.documentElement.classList.remove("is-home-route");
  }, [isHome]);

  const openNotifications = () => {
    setNotificationsOpen(true);
    window.setTimeout(() => dispatch({ type: "MARK_NOTIFICATIONS_READ" }), 350);
  };

  return (
    <div className={`app-shell ${isHome ? "app-shell--home" : "app-shell--inner"}`}>
      <IceFlowBackground variant={isHome ? "hero" : "subtle"} />
      <header className="site-header">
        <div className="site-header__inner">
          <Link to="/" className="wordmark" aria-label="WowSkillsLand 首页">
            WowSkills<span>Land</span>
          </Link>

          <nav className="desktop-nav" aria-label="主导航">
            {navigation.map((item) => (
              <NavLink key={item.to} to={item.to} className={({ isActive }) => `nav-link${isActive ? " is-active" : ""}`}>
                {item.label}
              </NavLink>
            ))}
          </nav>

          {isHome ? (
            <div className="home-header-actions">
              {state.isLoggedIn ? (
                <Link className="home-account-link" to={`/u/${currentUser.handle}`}>个人中心</Link>
              ) : (
                <button className="home-account-link" type="button" onClick={() => loginDialogRef.current?.showModal()}>登录</button>
              )}
              <Link className="home-publish-link" to="/creator">发布 Skill</Link>
            </div>
          ) : (
            <div className="header-actions">
              <button className="icon-button notification-button" type="button" onClick={openNotifications} aria-label={`通知，${unread} 条未读`}>
                {unread > 0 ? <BellRing size={18} /> : <Bell size={18} />}
                {unread > 0 && <span className="notification-dot">{unread}</span>}
              </button>
              {state.isLoggedIn ? (
                <div className="account-menu-wrap">
                  <button className="avatar-button" type="button" onClick={() => setAccountOpen((value) => !value)} aria-expanded={accountOpen} aria-label="打开账号菜单">
                    <Avatar user={currentUser} size="sm" />
                    <span>{currentUser.name}</span>
                  </button>
                  {accountOpen && (
                    <div className="account-menu">
                      <Link to={`/u/${currentUser.handle}`}><UserRound size={16} />公开主页</Link>
                      <button type="button" onClick={() => dispatch({ type: "SET_LOGIN", value: false })}><LogOut size={16} />退出演示账号</button>
                      <button type="button" onClick={() => dispatch({ type: "RESET_DEMO" })}><RotateCcw size={16} />重置演示数据</button>
                    </div>
                  )}
                </div>
              ) : (
                <button className="button button--dark button--small" type="button" onClick={() => loginDialogRef.current?.showModal()}>
                  登录
                </button>
              )}
              <button className="icon-button mobile-menu-button" type="button" onClick={() => setMobileOpen((value) => !value)} aria-label="打开导航" aria-expanded={mobileOpen}>
                {mobileOpen ? <X size={20} /> : <Menu size={20} />}
              </button>
            </div>
          )}
        </div>

        {mobileOpen && (
          <nav className="mobile-nav" aria-label="移动端导航">
            {navigation.map((item) => <NavLink key={item.to} to={item.to}>{item.label}</NavLink>)}
            <Link to={`/u/${currentUser.handle}`}>个人主页</Link>
          </nav>
        )}
      </header>

      <main className="app-main">
        <Outlet context={{ openLogin: () => loginDialogRef.current?.showModal() }} />
      </main>

      {notificationsOpen && (
        <div className="drawer-layer" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && setNotificationsOpen(false)}>
          <aside className="notification-drawer" aria-label="通知中心">
            <header>
              <div>
                <span className="eyebrow">动态同步</span>
                <h2>通知</h2>
              </div>
              <button className="icon-button" type="button" onClick={() => setNotificationsOpen(false)} aria-label="关闭通知"><X size={20} /></button>
            </header>
            <div className="notification-list">
              {state.social.notifications.length === 0 ? (
                <div className="empty-state"><Bell size={24} /><p>暂时没有新通知</p></div>
              ) : state.social.notifications.map((item) => (
                <Link key={item.id} to={item.href ?? "/explore"} className={`notification-item${item.read ? "" : " is-unread"}`}>
                  <span className="notification-item__icon"><Check size={14} /></span>
                  <span><strong>{item.message}</strong><small>{formatRelative(item.createdAt)}</small></span>
                </Link>
              ))}
            </div>
          </aside>
        </div>
      )}

      <dialog ref={loginDialogRef} className="modal login-modal" onClick={(event) => event.target === event.currentTarget && loginDialogRef.current?.close()}>
        <button className="modal__close icon-button" type="button" onClick={() => loginDialogRef.current?.close()} aria-label="关闭登录弹窗"><X size={20} /></button>
        <span className="eyebrow">演示账号</span>
        <h2>登录后继续参与</h2>
        <p>第一版不接真实账号。登录仅用于演示评论、关注、人格对话和跨页面状态同步。</p>
        <button
          className="button button--primary button--block"
          type="button"
          onClick={() => {
            dispatch({ type: "SET_LOGIN", value: true });
            loginDialogRef.current?.close();
          }}
        >
          以林夏的演示账号登录
        </button>
        <small>不会提交个人信息，也不会调用真实身份认证服务。</small>
      </dialog>
    </div>
  );
}

function formatRelative(value: string) {
  const distance = Date.now() - new Date(value).getTime();
  if (distance < 60 * 60 * 1000) return "刚刚";
  if (distance < 24 * 60 * 60 * 1000) return `${Math.max(1, Math.floor(distance / 3600000))} 小时前`;
  return new Intl.DateTimeFormat("zh-CN", { month: "numeric", day: "numeric" }).format(new Date(value));
}
