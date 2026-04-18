import * as React from 'react';
import { Title, Label } from '@patternfly/react-core';
import { A2AEventRecord } from '../../utils/a2a-types';

interface EventPanelProps {
  events: A2AEventRecord[];
}

const KIND_COLORS: Record<string, 'blue' | 'green' | 'orange' | 'purple' | 'grey'> = {
  'task': 'blue',
  'status-update': 'orange',
  'artifact-update': 'green',
  'message': 'purple',
};

const EventPanel: React.FC<EventPanelProps> = ({ events }) => {
  const [expandedIds, setExpandedIds] = React.useState<Set<string>>(new Set());

  const toggleExpand = React.useCallback((id: string) => {
    setExpandedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  return (
    <div className="gpu-agent-events">
      <Title headingLevel="h3" size="md" style={{ marginBottom: '12px' }}>
        Events
      </Title>
      {events.length === 0 && (
        <div style={{ fontSize: '13px', color: 'var(--pf-t--global--text--color--regular)', opacity: 0.5 }}>
          Events will appear here as the agent processes your request.
        </div>
      )}
      {events.map((event) => {
        const isExpanded = expandedIds.has(event.id);
        return (
          <div key={event.id} className="gpu-agent-event-item">
            <div
              className="gpu-agent-event-header"
              onClick={() => toggleExpand(event.id)}
            >
              <Label color={KIND_COLORS[event.kind] || 'grey'} isCompact>
                {event.kind}
              </Label>
              <span style={{ fontSize: '11px', opacity: 0.7 }}>
                {event.timestamp.toLocaleTimeString()}
              </span>
            </div>
            {isExpanded && (
              <pre className="gpu-agent-event-json">
                {JSON.stringify(event.raw, null, 2)}
              </pre>
            )}
          </div>
        );
      })}
    </div>
  );
};

export default EventPanel;
