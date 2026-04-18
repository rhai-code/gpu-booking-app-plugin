import * as React from 'react';
import { Title } from '@patternfly/react-core';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeRaw from 'rehype-raw';
import { ChatMessage as ChatMessageType } from '../../utils/a2a-types';
import ToolCallCard from './ToolCallCard';

interface ChatMessageProps {
  message: ChatMessageType;
}

const mdComponents: Record<string, React.FC<any>> = {
  h1: ({ children }: { children?: React.ReactNode }) => (
    <Title headingLevel="h3" size="lg" style={{ marginBottom: '8px', marginTop: '12px' }}>
      {children}
    </Title>
  ),
  h2: ({ children }: { children?: React.ReactNode }) => (
    <Title headingLevel="h4" size="md" style={{ marginBottom: '6px', marginTop: '10px' }}>
      {children}
    </Title>
  ),
  h3: ({ children }: { children?: React.ReactNode }) => (
    <Title headingLevel="h5" style={{ marginBottom: '4px', marginTop: '8px' }}>
      {children}
    </Title>
  ),
  code: ({ className, children }: { className?: string; children?: React.ReactNode }) => {
    const isBlock = className?.startsWith('language-') || (typeof children === 'string' && children.includes('\n'));
    if (isBlock) {
      return (
        <pre className="gpu-help-codeblock">
          <code className={className}>{children}</code>
        </pre>
      );
    }
    return <code className="gpu-help-inline-code">{children}</code>;
  },
  pre: ({ children }: { children?: React.ReactNode }) => <>{children}</>,
  table: ({ children, ...props }: { children?: React.ReactNode }) => (
    <div style={{ overflowX: 'auto', marginBottom: '12px' }}>
      <table {...props} className="gpu-help-table">{children}</table>
    </div>
  ),
};

const ChatMessageComponent: React.FC<ChatMessageProps> = ({ message }) => {
  const isUser = message.role === 'user';
  const className = `gpu-agent-message ${isUser ? 'gpu-agent-message--user' : 'gpu-agent-message--agent'}`;

  return (
    <div className={className}>
      <div className="gpu-agent-message-header">
        <span>{isUser ? 'You' : 'GPU Booking Agent'}</span>
        <span style={{ fontSize: '11px', fontWeight: 400 }}>
          {message.timestamp.toLocaleTimeString()}
        </span>
      </div>
      <div className="gpu-agent-message-body">
        <ReactMarkdown
          remarkPlugins={[remarkGfm]}
          rehypePlugins={[rehypeRaw]}
          components={mdComponents}
        >
          {message.text}
        </ReactMarkdown>
      </div>
      {message.artifacts && message.artifacts.length > 0 && message.artifacts.map((artifact) =>
        artifact.parts
          .filter((p) => p.kind === 'data' && p.data)
          .map((p, idx) => (
            <ToolCallCard key={`${artifact.artifactId}-${idx}`} data={p.data as Record<string, unknown>} />
          )),
      )}
    </div>
  );
};

export default ChatMessageComponent;
