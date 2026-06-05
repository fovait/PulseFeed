import { Fragment, type ReactNode } from "react";
import { Link } from "react-router-dom";

// 后端 ExtractTags 的语义：以 # 开头、连续的非空白非 # 字符。这里前端只做展示分割，匹配上 #foo / #中文标签 即可。
const TAG_REGEX = /#([^\s#]+)/g;

export function renderWithTags(text?: string): ReactNode {
  if (!text) return null;
  const parts: ReactNode[] = [];
  let lastIndex = 0;
  let match: RegExpExecArray | null;
  TAG_REGEX.lastIndex = 0;
  while ((match = TAG_REGEX.exec(text)) !== null) {
    if (match.index > lastIndex) {
      parts.push(text.slice(lastIndex, match.index));
    }
    const tag = match[1];
    parts.push(
      <Link
        key={`${tag}-${match.index}`}
        to={`/tag/${encodeURIComponent(tag)}`}
        className="text-pulse-cyan hover:underline"
      >
        #{tag}
      </Link>,
    );
    lastIndex = match.index + match[0].length;
  }
  if (lastIndex < text.length) {
    parts.push(text.slice(lastIndex));
  }
  return parts.map((node, i) => <Fragment key={i}>{node}</Fragment>);
}
