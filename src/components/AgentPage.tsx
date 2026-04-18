import * as React from 'react';
import { Helmet } from 'react-helmet';
import {
  PageSection,
  Title,
  Button,
  Alert,
  Spinner,
  Bullseye,
  Toolbar,
  ToolbarContent,
  ToolbarItem,
  Label,
} from '@patternfly/react-core';
import { EraserIcon } from '@patternfly/react-icons';
import { useAuth } from '../utils/AuthContext';
import { fetchAgentCard, streamMessage } from '../utils/a2a-client';
import {
  A2AAgentCard,
  A2AEvent,
  A2ATask,
  A2AStatusUpdate,
  A2AArtifactUpdate,
  A2ADirectMessage,
  ChatMessage,
  A2AEventRecord,
} from '../utils/a2a-types';
import ChatPanel from './agent/ChatPanel';
import EventPanel from './agent/EventPanel';
import './styles.css';

function generateId(): string {
  const arr = new Uint8Array(16);
  window.crypto.getRandomValues(arr);
  return Array.from(arr, (b) => b.toString(16).padStart(2, '0')).join('').slice(0, 12);
}

function extractText(event: A2AEvent): string {
  if (event.kind === 'task') {
    const task = event as A2ATask;
    const statusText = task.status?.message?.parts
      ?.filter((p) => p.kind === 'text' && p.text)
      .map((p) => p.text)
      .join('\n') || '';
    const artifactText = (task.artifacts || [])
      .flatMap((a) => a.parts)
      .filter((p) => p.kind === 'text' && p.text)
      .map((p) => p.text)
      .join('\n');
    return [statusText, artifactText].filter(Boolean).join('\n\n');
  }
  if (event.kind === 'status-update') {
    const update = event as A2AStatusUpdate;
    return update.status?.message?.parts
      ?.filter((p) => p.kind === 'text' && p.text)
      .map((p) => p.text)
      .join('\n') || '';
  }
  if (event.kind === 'artifact-update') {
    const update = event as A2AArtifactUpdate;
    return update.artifact?.parts
      ?.filter((p) => p.kind === 'text' && p.text)
      .map((p) => p.text)
      .join('\n') || '';
  }
  if (event.kind === 'message') {
    const msg = event as A2ADirectMessage;
    return msg.parts
      ?.filter((p) => p.kind === 'text' && p.text)
      .map((p) => p.text)
      .join('\n') || '';
  }
  return '';
}

function extractTaskId(event: A2AEvent): string | undefined {
  if ('taskId' in event) return (event as A2AStatusUpdate).taskId;
  if ('id' in event && event.kind === 'task') return (event as A2ATask).id;
  return undefined;
}

function extractContextId(event: A2AEvent): string | undefined {
  if ('contextId' in event) return (event as A2ATask).contextId;
  return undefined;
}

const AgentPage: React.FC = () => {
  useAuth();
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [agentCard, setAgentCard] = React.useState<A2AAgentCard | null>(null);
  const [messages, setMessages] = React.useState<ChatMessage[]>([]);
  const [events, setEvents] = React.useState<A2AEventRecord[]>([]);
  const [isStreaming, setIsStreaming] = React.useState(false);
  const contextIdRef = React.useRef<string | undefined>(undefined);
  const taskIdRef = React.useRef<string | undefined>(undefined);

  React.useEffect(() => {
    fetchAgentCard()
      .then((card) => {
        setAgentCard(card);
        if (!card) setError('Could not connect to AI agent. Is the agent service deployed?');
      })
      .catch(() => setError('Could not connect to AI agent'))
      .finally(() => setLoading(false));
  }, []);

  const handleClear = React.useCallback(() => {
    setMessages([]);
    setEvents([]);
    setError(null);
    contextIdRef.current = undefined;
    taskIdRef.current = undefined;
  }, []);

  const handleSend = React.useCallback(
    (text: string) => {
      const userMsg: ChatMessage = {
        id: generateId(),
        role: 'user',
        text,
        timestamp: new Date(),
      };
      setMessages((prev) => [...prev, userMsg]);
      setIsStreaming(true);
      setError(null);

      const agentMsgId = generateId();
      let agentText = '';

      streamMessage(
        text,
        (event: A2AEvent) => {
          const newContextId = extractContextId(event);
          if (newContextId) contextIdRef.current = newContextId;

          const newTaskId = extractTaskId(event);
          if (newTaskId) taskIdRef.current = newTaskId;

          const eventRecord: A2AEventRecord = {
            id: generateId(),
            timestamp: new Date(),
            kind: event.kind,
            raw: event,
            taskId: newTaskId,
            linkedMessageId: agentMsgId,
          };
          setEvents((prev) => [...prev, eventRecord]);

          const text = extractText(event);
          if (text) {
            agentText = agentText ? agentText + '\n\n' + text : text;
            setMessages((prev) => {
              const existing = prev.find((m) => m.id === agentMsgId);
              if (existing) {
                return prev.map((m) =>
                  m.id === agentMsgId ? { ...m, text: agentText, isStreaming: true } : m,
                );
              }
              return [
                ...prev,
                {
                  id: agentMsgId,
                  role: 'agent' as const,
                  text: agentText,
                  timestamp: new Date(),
                  taskId: newTaskId,
                  isStreaming: true,
                },
              ];
            });
          }
        },
        () => {
          setIsStreaming(false);
          setMessages((prev) =>
            prev.map((m) => (m.id === agentMsgId ? { ...m, isStreaming: false } : m)),
          );
        },
        (err: string) => {
          setIsStreaming(false);
          setError(err);
          setMessages((prev) =>
            prev.map((m) => (m.id === agentMsgId ? { ...m, isStreaming: false } : m)),
          );
        },
        contextIdRef.current,
        taskIdRef.current,
      );
    },
    [],
  );

  if (loading) {
    return (
      <Bullseye>
        <Spinner size="xl" />
      </Bullseye>
    );
  }

  return (
    <>
      <Helmet>
        <title>GPU Booking - GPU Booking Agent</title>
      </Helmet>
      <>
        <PageSection style={{ paddingBottom: '24px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
            <div>
              <Title headingLevel="h1" size="2xl">
                GPU Booking Agent
              </Title>
              <div style={{ color: 'var(--pf-t--global--text--color--regular)', opacity: 0.7, marginTop: '8px' }}>
                Book GPU resources using natural language
                {agentCard && (
                  <Label color="green" isCompact style={{ marginLeft: '8px' }}>
                    Connected: {agentCard.name}
                  </Label>
                )}
              </div>
            </div>
            <Toolbar>
              <ToolbarContent>
                <ToolbarItem>
                  <Button
                    variant="secondary"
                    onClick={handleClear}
                    icon={<EraserIcon />}
                  >
                    New Chat
                  </Button>
                </ToolbarItem>
              </ToolbarContent>
            </Toolbar>
          </div>
        </PageSection>

        <PageSection>
          {error && (
            <Alert
              variant="danger"
              title={error}
              isInline
              actionClose={<Button variant="plain" onClick={() => setError(null)}>&times;</Button>}
              style={{ marginBottom: '16px' }}
            />
          )}

          <div className="gpu-agent-layout">
            <ChatPanel messages={messages} onSend={handleSend} isStreaming={isStreaming} />
            <EventPanel events={events} />
          </div>
        </PageSection>
      </>
    </>
  );
};

export default AgentPage;
