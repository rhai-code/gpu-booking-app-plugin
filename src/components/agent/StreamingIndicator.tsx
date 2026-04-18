import * as React from 'react';
import { Spinner } from '@patternfly/react-core';

const StreamingIndicator: React.FC = () => (
  <div className="gpu-agent-message gpu-agent-message--agent">
    <div className="gpu-agent-streaming">
      <Spinner size="md" />
      <span>Agent is thinking...</span>
    </div>
  </div>
);

export default StreamingIndicator;
