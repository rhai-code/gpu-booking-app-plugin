import * as React from 'react';
import { Button, TextInput } from '@patternfly/react-core';
import { PaperPlaneIcon, RobotIcon } from '@patternfly/react-icons';
import { ChatMessage as ChatMessageType } from '../../utils/a2a-types';
import ChatMessageComponent from './ChatMessage';
import StreamingIndicator from './StreamingIndicator';

interface ChatPanelProps {
  messages: ChatMessageType[];
  onSend: (text: string) => void;
  isStreaming: boolean;
}

const ChatPanel: React.FC<ChatPanelProps> = ({ messages, onSend, isStreaming }) => {
  const [input, setInput] = React.useState('');
  const messagesEndRef = React.useRef<HTMLDivElement>(null);

  React.useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, isStreaming]);

  const handleSubmit = React.useCallback(() => {
    const text = input.trim();
    if (!text || isStreaming) return;
    setInput('');
    onSend(text);
  }, [input, isStreaming, onSend]);

  const handleKeyDown = React.useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        handleSubmit();
      }
    },
    [handleSubmit],
  );

  return (
    <div className="gpu-agent-chat">
      <div className="gpu-agent-messages">
        {messages.length === 0 && !isStreaming && (
          <div className="gpu-agent-empty">
            <RobotIcon className="gpu-agent-empty-icon" />
            <div style={{ fontSize: '16px', fontWeight: 600, marginBottom: '8px' }}>
              GPU Booking Agent
            </div>
            <div>Ask me to check GPU availability, make bookings, or manage reservations.</div>
          </div>
        )}
        {messages.map((msg) => (
          <ChatMessageComponent key={msg.id} message={msg} />
        ))}
        {isStreaming && <StreamingIndicator />}
        <div ref={messagesEndRef} />
      </div>
      <div className="gpu-agent-input">
        <TextInput
          className="gpu-agent-input-field"
          type="text"
          aria-label="Message input"
          placeholder={isStreaming ? 'Waiting for response...' : 'Ask about GPU bookings...'}
          value={input}
          onChange={(_e, val) => setInput(val)}
          onKeyDown={handleKeyDown}
          isDisabled={isStreaming}
        />
        <Button
          variant="primary"
          icon={<PaperPlaneIcon />}
          onClick={handleSubmit}
          isDisabled={!input.trim() || isStreaming}
          aria-label="Send message"
        />
      </div>
    </div>
  );
};

export default ChatPanel;
