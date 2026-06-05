import { Bell, CheckCheck, MessageCircle, Search, SendHorizonal } from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";
import { useSearchParams } from "react-router-dom";
import { pulsefeedApi } from "../api/pulsefeed";
import { useAuth } from "../hooks/useAuth";
import { useNotifications } from "../hooks/useNotifications";
import { useToast } from "../hooks/useToast";
import { requestUnreadRefresh } from "../hooks/useUnreadSummary";
import type { Account, Message, MessageConversation } from "../types/api";
import { formatRelativeTime } from "../utils/time";

const ACTIVE_MESSAGE_POLL_MS = 5_000;

export function MessagesPage() {
  const [searchParams] = useSearchParams();
  const [peerQuery, setPeerQuery] = useState(searchParams.get("peer_id") || "");
  const [activePeerID, setActivePeerID] = useState<number | null>(null);
  const [searchedPeer, setSearchedPeer] = useState<Account | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [conversations, setConversations] = useState<MessageConversation[]>([]);
  const [conversationUnreadCount, setConversationUnreadCount] = useState(0);
  const [nextBeforeID, setNextBeforeID] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [content, setContent] = useState("");
  const [loadingConversations, setLoadingConversations] = useState(false);
  const [searchingPeer, setSearchingPeer] = useState(false);
  const { session, requireAuth, openAuth } = useAuth();
  const { pushToast } = useToast();
  const notifications = useNotifications(session);

  useEffect(() => {
    if (!session?.token) {
      setConversations([]);
      setConversationUnreadCount(0);
      return;
    }
    refreshConversations();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session?.token]);

  useEffect(() => {
    const fromQuery = Number(searchParams.get("peer_id") || 0);
    if (fromQuery > 0) {
      setPeerQuery(String(fromQuery));
      loadMessages(fromQuery, false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchParams]);

  useEffect(() => {
    if (activePeerID || peerQuery || conversations.length === 0) return;
    loadMessages(conversations[0].peer_id, false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activePeerID, conversations, peerQuery]);

  useEffect(() => {
    if (!session?.token || !activePeerID) return undefined;

    const refreshSilently = () => {
      refreshActiveMessages(true).catch(() => undefined);
    };
    const refreshWhenVisible = () => {
      if (document.visibilityState === "visible") {
        refreshSilently();
      }
    };
    const intervalID = window.setInterval(refreshSilently, ACTIVE_MESSAGE_POLL_MS);

    window.addEventListener("focus", refreshSilently);
    document.addEventListener("visibilitychange", refreshWhenVisible);

    return () => {
      window.clearInterval(intervalID);
      window.removeEventListener("focus", refreshSilently);
      document.removeEventListener("visibilitychange", refreshWhenVisible);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activePeerID, session?.token]);

  async function refreshConversations(silent = false) {
    if (!session?.token) return;
    if (!silent) {
      setLoadingConversations(true);
    }
    try {
      const response = await pulsefeedApi.listMessageConversations();
      setConversations(response.conversations || []);
      setConversationUnreadCount(response.unread_count || 0);
      requestUnreadRefresh();
    } catch (error) {
      pushToast(error instanceof Error ? error.message : "私信会话加载失败", "error");
    } finally {
      if (!silent) {
        setLoadingConversations(false);
      }
    }
  }

  async function refreshActiveMessages(silent = false) {
    if (!session?.token || !activePeerID) return;
    try {
      const response = await pulsefeedApi.listMessages(activePeerID, 20, 0);
      const latest = response.messages || [];
      setMessages((items) => mergeMessages(items, latest));
      if (!nextBeforeID) {
        setNextBeforeID(response.next_before_id || 0);
        setHasMore(Boolean(response.has_more));
      }
      await refreshConversations(true);
    } catch (error) {
      if (!silent) {
        pushToast(error instanceof Error ? error.message : "私信刷新失败", "error");
      }
    }
  }

  async function refreshCurrentView() {
    if (activePeerID) {
      await refreshActiveMessages(false);
      return;
    }
    await refreshConversations(false);
  }

  async function loadMessages(targetPeerID = Number(peerQuery), append = false) {
    if (!requireAuth("登录后才能查看私信")) return;
    if (!targetPeerID) {
      pushToast("请输入用户名或 ID", "error");
      return;
    }
    try {
      const response = await pulsefeedApi.listMessages(targetPeerID, 20, append ? nextBeforeID : 0);
      setActivePeerID(targetPeerID);
      setPeerQuery(String(targetPeerID));
      setMessages((items) => (append ? [...(response.messages || []), ...items] : response.messages || []));
      setNextBeforeID(response.next_before_id || 0);
      setHasMore(Boolean(response.has_more));
      await refreshConversations();
    } catch (error) {
      pushToast(error instanceof Error ? error.message : "私信加载失败", "error");
    }
  }

  async function findAndLoadPeer(event: FormEvent) {
    event.preventDefault();
    if (!requireAuth("登录后才能发起私信")) return;
    const query = peerQuery.trim().replace(/^@/, "");
    if (!query) {
      pushToast("请输入用户名或 ID", "error");
      return;
    }
    if (/^\d+$/.test(query)) {
      setSearchedPeer(null);
      await loadMessages(Number(query), false);
      return;
    }

    setSearchingPeer(true);
    try {
      const account = await pulsefeedApi.findAccountByUsername(query);
      if (account.id === session?.account_id) {
        pushToast("不能给自己发私信", "error");
        return;
      }
      setSearchedPeer(account);
      await loadMessages(account.id, false);
      setPeerQuery(account.username);
    } catch (error) {
      pushToast(error instanceof Error ? error.message : "用户查询失败", "error");
    } finally {
      setSearchingPeer(false);
    }
  }

  async function send(event: FormEvent) {
    event.preventDefault();
    if (!requireAuth("登录后才能发送私信")) return;
    const toID = activePeerID || Number(peerQuery);
    const text = content.trim();
    if (!toID || !text) return;
    try {
      const msg = await pulsefeedApi.sendMessage(toID, text);
      setMessages((items) => [...items, msg]);
      setActivePeerID(toID);
      setPeerQuery(String(toID));
      setContent("");
      await refreshConversations();
    } catch (error) {
      pushToast(error instanceof Error ? error.message : "发送失败", "error");
    }
  }

  async function markNotificationsRead(id?: number) {
    await notifications.markRead(id);
    requestUnreadRefresh();
  }

  const activeConversation = conversations.find((conversation) => conversation.peer_id === activePeerID);
  const activePeerLabel = activePeerID
    ? `@${activeConversation?.peer_username || (searchedPeer?.id === activePeerID ? searchedPeer.username : `peer ${activePeerID}`)}`
    : "选择会话";

  return (
    <main className="min-h-[100svh] bg-pulse-black px-4 pb-28 pt-5 md:h-[100svh] md:overflow-hidden md:pb-8 md:pl-28 md:pr-8 md:pt-8">
      <div className="mx-auto flex w-full max-w-[1120px] flex-col md:h-full">
        <header className="mb-5 flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-black md:text-3xl">消息</h1>
            <p className="mt-1 text-sm text-white/52">通知和私信</p>
          </div>
          {!session?.token ? (
            <button className="primary-button" onClick={() => openAuth("登录后查看消息")}>
              登录
            </button>
          ) : null}
        </header>

        <div className="grid gap-4 md:min-h-0 md:flex-1 lg:grid-cols-[360px_minmax(0,1fr)]">
          <section className="glass-panel flex min-h-[420px] flex-col rounded-lg p-4 md:min-h-0 lg:h-full">
            <div className="border-b border-white/10 pb-4">
              <div className="mb-3 flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Bell className="h-5 w-5 text-pulse-cyan" />
                  <h2 className="font-black">通知</h2>
                  <span className="rounded-lg bg-pulse-red px-2 py-0.5 text-xs font-black">{notifications.unreadCount}</span>
                </div>
                <button className="ghost-button flex items-center gap-1" onClick={() => markNotificationsRead()}>
                  <CheckCheck className="h-4 w-4" />
                  全部已读
                </button>
              </div>
              <div className="max-h-36 space-y-2 overflow-y-auto">
                {notifications.notifications.length === 0 ? (
                  <p className="text-sm text-white/52">暂无通知</p>
                ) : (
                  notifications.notifications.map((item) => (
                    <button
                      key={item.id}
                      className="w-full rounded-lg bg-white/[0.06] p-3 text-left"
                      onClick={() => markNotificationsRead(item.id)}
                    >
                      <div className="flex items-center justify-between gap-3">
                        <p className="text-sm font-bold">{item.content || item.type}</p>
                        {!item.is_read ? <span className="h-2 w-2 rounded-full bg-pulse-red" /> : null}
                      </div>
                      <p className="mt-1 text-xs text-white/42">
                        sender #{item.sender_id} · target #{item.target_id} · {formatRelativeTime(item.created_at)}
                      </p>
                    </button>
                  ))
                )}
              </div>
            </div>

            <div className="mt-4 flex min-h-0 flex-1 flex-col">
              <div className="mb-3 flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <MessageCircle className="h-5 w-5 text-pulse-cyan" />
                  <h2 className="font-black">私信</h2>
                  <span className="rounded-lg bg-pulse-red px-2 py-0.5 text-xs font-black">{conversationUnreadCount}</span>
                </div>
                {loadingConversations ? <span className="text-xs text-white/42">同步中</span> : null}
              </div>

              <div className="min-h-0 flex-1 space-y-2 overflow-y-auto">
                {conversations.length === 0 ? (
                  <p className="text-sm text-white/52">暂无私信会话</p>
                ) : (
                  conversations.map((conversation) => {
                    const active = conversation.peer_id === activePeerID;
                    const mine = conversation.last_message.from_id === session?.account_id;
                    return (
                      <button
                        key={conversation.peer_id}
                        className={[
                          "w-full rounded-lg p-3 text-left transition",
                          active ? "bg-pulse-cyan text-black" : "bg-white/[0.06] hover:bg-white/[0.1]",
                        ].join(" ")}
                        onClick={() => {
                          setSearchedPeer(null);
                          loadMessages(conversation.peer_id, false);
                        }}
                      >
                        <div className="flex items-center justify-between gap-3">
                          <p className="truncate text-sm font-black">
                            @{conversation.peer_username || `peer ${conversation.peer_id}`}
                          </p>
                          {conversation.unread_count > 0 ? (
                            <span
                              className={[
                                "rounded-lg px-2 py-0.5 text-xs font-black",
                                active ? "bg-black text-white" : "bg-pulse-red text-white",
                              ].join(" ")}
                            >
                              {conversation.unread_count}
                            </span>
                          ) : null}
                        </div>
                        <p className={["mt-1 truncate text-xs", active ? "text-black/64" : "text-white/52"].join(" ")}>
                          {mine ? "我：" : ""}
                          {conversation.last_message.content}
                        </p>
                        <p className={["mt-1 text-[0.68rem]", active ? "text-black/50" : "text-white/36"].join(" ")}>
                          {formatRelativeTime(conversation.updated_at || conversation.last_message.created_at)}
                        </p>
                      </button>
                    );
                  })
                )}
              </div>

              <div className="mt-4 border-t border-white/10 pt-4">
                <p className="mb-2 text-xs font-bold uppercase tracking-[0.08em] text-white/42">新会话</p>
                <form className="flex gap-2" onSubmit={findAndLoadPeer}>
                  <input
                    className="control-field"
                    value={peerQuery}
                    onChange={(event) => setPeerQuery(event.target.value)}
                    placeholder="用户名或 ID"
                  />
                  <button className="ghost-button flex shrink-0 items-center gap-1" disabled={searchingPeer}>
                    <Search className="h-4 w-4" />
                    {searchingPeer ? "查询中" : "查询"}
                  </button>
                </form>
              </div>
            </div>
          </section>

          <section className="glass-panel flex min-h-[540px] flex-col rounded-lg p-4 md:min-h-0 lg:h-full">
            <div className="mb-4 flex items-center justify-between gap-3">
              <div>
                <h2 className="font-black">私信</h2>
                <p className="mt-1 text-xs text-white/42">{activePeerLabel}</p>
              </div>
              <button className="ghost-button shrink-0" onClick={refreshCurrentView}>
                刷新
              </button>
            </div>

            <div className="mb-3 min-h-72 flex-1 space-y-2 overflow-y-auto rounded-lg bg-black/42 p-3 md:min-h-0">
              {hasMore ? (
                <button className="ghost-button mx-auto block text-xs" onClick={() => loadMessages(activePeerID || Number(peerQuery), true)}>
                  加载更早
                </button>
              ) : null}
              {messages.length === 0 ? (
                <div className="grid h-full min-h-72 place-items-center text-center md:min-h-0">
                  <p className="text-sm text-white/52">从左侧选择会话，或输入用户名/ID 发起私信</p>
                </div>
              ) : (
                messages.map((message) => {
                  const mine = message.from_id === session?.account_id;
                  return (
                    <div key={message.id} className={["flex", mine ? "justify-end" : "justify-start"].join(" ")}>
                      <div
                        className={[
                          "max-w-[78%] rounded-lg px-3 py-2 text-sm leading-6 lg:max-w-[58%]",
                          mine ? "bg-pulse-cyan text-black" : "bg-white/10 text-white",
                        ].join(" ")}
                      >
                        <p>{message.content}</p>
                        <p className={["mt-1 text-[0.68rem]", mine ? "text-black/58" : "text-white/42"].join(" ")}>
                          {formatRelativeTime(message.created_at)}
                        </p>
                      </div>
                    </div>
                  );
                })
              )}
            </div>

            <form className="flex items-end gap-2" onSubmit={send}>
              <textarea
                className="control-field min-h-12 resize-none"
                value={content}
                onChange={(event) => setContent(event.target.value)}
                maxLength={1000}
                placeholder={activePeerID || Number(peerQuery) ? "输入私信内容" : "先选择会话"}
              />
              <button
                className="grid h-12 w-12 shrink-0 place-items-center rounded-lg bg-pulse-cyan text-black disabled:opacity-45"
                disabled={!content.trim() || !(activePeerID || Number(peerQuery))}
              >
                <SendHorizonal className="h-5 w-5" />
              </button>
            </form>
          </section>
        </div>
      </div>
    </main>
  );
}

function mergeMessages(current: Message[], latest: Message[]) {
  const byID = new Map<number, Message>();
  for (const message of current) {
    byID.set(message.id, message);
  }
  for (const message of latest) {
    byID.set(message.id, message);
  }
  return Array.from(byID.values()).sort((a, b) => a.id - b.id);
}
