type MessageBubbleProps = {
  role: "user" | "assistant";
  roleLabel: string;
  avatarLabel: string;
  content: string;
};

export function MessageBubble({
  role,
  roleLabel,
  avatarLabel,
  content,
}: MessageBubbleProps) {
  const paragraphs = splitIntoParagraphs(content);

  return (
    <article className={role === "user" ? "chat-row chat-row-user" : "chat-row chat-row-assistant"}>
      {role === "assistant" ? (
        <div className="chat-avatar border border-white/6 bg-zinc-800/90">{avatarLabel}</div>
      ) : null}
      <div className="chat-bubble-wrap">
        <span className="chat-bubble-role">{roleLabel}</span>
        <div
          className={
            role === "user"
              ? "chat-bubble chat-bubble-user border border-white/6 shadow-[0_12px_32px_rgba(0,0,0,0.12)]"
              : "chat-bubble chat-bubble-assistant bg-white/[0.02] shadow-[inset_0_1px_0_rgba(255,255,255,0.03)]"
          }
        >
          <div className={role === "assistant" ? "space-y-3 text-[1.02rem] leading-8 text-zinc-100" : "space-y-2 leading-7"}>
            {paragraphs.map((paragraph, index) => (
              <p key={index}>{paragraph}</p>
            ))}
          </div>
        </div>
      </div>
    </article>
  );
}

function splitIntoParagraphs(content: string) {
  const normalized = content.trim();
  if (!normalized) {
    return [""];
  }

  return normalized
    .split(/\n\s*\n/)
    .map((part) => part.trim())
    .filter(Boolean);
}
