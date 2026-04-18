import * as React from 'react';
import {
  Card,
  CardBody,
  CardTitle,
  DescriptionList,
  DescriptionListGroup,
  DescriptionListTerm,
  DescriptionListDescription,
  Label,
} from '@patternfly/react-core';

interface ToolCallCardProps {
  data: Record<string, unknown>;
}

function tryParseJson(value: unknown): Record<string, unknown> | null {
  if (typeof value === 'string') {
    try {
      const parsed = JSON.parse(value);
      if (typeof parsed === 'object' && parsed !== null) return parsed as Record<string, unknown>;
    } catch {
      // not JSON
    }
  }
  if (typeof value === 'object' && value !== null) return value as Record<string, unknown>;
  return null;
}

function renderValue(value: unknown): React.ReactNode {
  if (value === null || value === undefined) return <span style={{ opacity: 0.5 }}>—</span>;
  if (typeof value === 'boolean') {
    return <Label color={value ? 'green' : 'red'}>{String(value)}</Label>;
  }
  if (typeof value === 'object') return <pre className="gpu-agent-event-json">{JSON.stringify(value, null, 2)}</pre>;
  return String(value);
}

const ToolCallCard: React.FC<ToolCallCardProps> = ({ data }) => {
  const parsed = tryParseJson(data);
  if (!parsed) return null;

  const entries = Object.entries(parsed).filter(
    ([key]) => key !== 'kind' && key !== 'metadata',
  );

  if (entries.length === 0) return null;

  const title = typeof parsed.tool === 'string'
    ? parsed.tool
    : typeof parsed.name === 'string'
      ? parsed.name
      : 'Tool Result';

  return (
    <Card className="gpu-agent-tool-card" isCompact>
      <CardTitle>{title}</CardTitle>
      <CardBody>
        <DescriptionList isHorizontal isCompact>
          {entries.map(([key, val]) => (
            <DescriptionListGroup key={key}>
              <DescriptionListTerm>{key}</DescriptionListTerm>
              <DescriptionListDescription>{renderValue(val)}</DescriptionListDescription>
            </DescriptionListGroup>
          ))}
        </DescriptionList>
      </CardBody>
    </Card>
  );
};

export default ToolCallCard;
