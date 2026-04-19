import * as React from 'react';
import {
  Button,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  ModalVariant,
  FormGroup,
  FormHelperText,
  HelperText,
  HelperTextItem,
  TextInput,
  FormSelect,
  FormSelectOption,
  Switch,
  Alert,
  Spinner,
  Bullseye,
  EmptyState,
  EmptyStateBody,
  Label,
  Toolbar,
  ToolbarContent,
  ToolbarItem,
} from '@patternfly/react-core';
import { PlusCircleIcon, TrashIcon, PencilAltIcon, PluggedIcon, CogIcon } from '@patternfly/react-icons';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import {
  LLMEndpoint,
  LLMEndpointRequest,
  getLLMEndpoints,
  createLLMEndpoint,
  updateLLMEndpoint,
  deleteLLMEndpoint,
  testLLMEndpoint,
} from '../../utils/api';
import { useAuth } from '../../utils/AuthContext';

const PROVIDER_OPTIONS = [
  { value: 'openai-compatible', label: 'OpenAI Compatible (vLLM, Llama Stack, etc.)' },
  { value: 'rhoai-maas', label: 'RHOAI MaaS (Model as a Service)' },
  { value: 'gemini', label: 'Google Gemini' },
  { value: 'ollama', label: 'Ollama' },
];

interface EndpointFormData {
  name: string;
  url: string;
  api_key: string;
  model_name: string;
  provider_type: string;
  is_global: boolean;
  enabled: boolean;
}

const emptyForm: EndpointFormData = {
  name: '',
  url: '',
  api_key: '',
  model_name: '',
  provider_type: 'openai-compatible',
  is_global: false,
  enabled: true,
};

interface EndpointSettingsProps {
  isOpen: boolean;
  onClose: () => void;
  onEndpointsChanged?: () => void;
}

const EndpointSettings: React.FC<EndpointSettingsProps> = ({ isOpen, onClose, onEndpointsChanged }) => {
  const { isAdmin } = useAuth();
  const [endpoints, setEndpoints] = React.useState<LLMEndpoint[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [testResult, setTestResult] = React.useState<{ id: number; status: string; message?: string } | null>(null);

  const [formOpen, setFormOpen] = React.useState(false);
  const [editingId, setEditingId] = React.useState<number | null>(null);
  const [form, setForm] = React.useState<EndpointFormData>(emptyForm);
  const [saving, setSaving] = React.useState(false);

  const fetchEndpoints = React.useCallback(() => {
    setLoading(true);
    setError(null);
    getLLMEndpoints()
      .then((resp) => setEndpoints(resp.endpoints || []))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  React.useEffect(() => {
    if (isOpen) fetchEndpoints();
  }, [isOpen, fetchEndpoints]);

  const handleAdd = () => {
    setEditingId(null);
    setForm(emptyForm);
    setFormOpen(true);
  };

  const handleEdit = (ep: LLMEndpoint) => {
    setEditingId(ep.id);
    setForm({
      name: ep.name,
      url: ep.url,
      api_key: ep.api_key,
      model_name: ep.model_name,
      provider_type: ep.provider_type,
      is_global: ep.is_global,
      enabled: ep.enabled,
    });
    setFormOpen(true);
  };

  const handleDelete = (id: number) => {
    deleteLLMEndpoint(id)
      .then(() => {
        fetchEndpoints();
        onEndpointsChanged?.();
      })
      .catch((err) => setError(err.message));
  };

  const handleTest = (id: number) => {
    setTestResult(null);
    testLLMEndpoint(id)
      .then((result) => setTestResult({ id, ...result }))
      .catch((err) => setTestResult({ id, status: 'error', message: err.message }));
  };

  const handleSave = () => {
    setSaving(true);
    setError(null);

    const req: LLMEndpointRequest = {
      name: form.name,
      url: form.url,
      api_key: form.api_key,
      model_name: form.model_name,
      provider_type: form.provider_type,
      is_global: form.is_global,
    };

    const promise = editingId
      ? updateLLMEndpoint(editingId, { ...req, enabled: form.enabled })
      : createLLMEndpoint(req);

    promise
      .then(() => {
        setFormOpen(false);
        fetchEndpoints();
        onEndpointsChanged?.();
      })
      .catch((err) => setError(err.message))
      .finally(() => setSaving(false));
  };

  const updateField = <K extends keyof EndpointFormData>(field: K, value: EndpointFormData[K]) => {
    setForm((prev) => ({ ...prev, [field]: value }));
  };

  if (!isOpen) return null;

  return (
    <Modal
      variant={ModalVariant.large}
      isOpen={isOpen}
      onClose={onClose}
      aria-label="LLM Endpoint Settings"
    >
      <ModalHeader title="LLM Endpoint Configuration" />
      <ModalBody>
        {error && (
          <Alert
            variant="danger"
            title={error}
            isInline
            actionClose={<Button variant="plain" onClick={() => setError(null)}>&times;</Button>}
            style={{ marginBottom: '16px' }}
          />
        )}

        {formOpen ? (
          <div>
            <FormGroup label="Name" isRequired fieldId="ep-name">
              <TextInput
                id="ep-name"
                value={form.name}
                onChange={(_e, v) => updateField('name', v)}
                placeholder="e.g. Team Gemini, My Llama Stack"
              />
            </FormGroup>
            <FormGroup label="Provider Type" isRequired fieldId="ep-provider" style={{ marginTop: '12px' }}>
              <FormSelect
                id="ep-provider"
                value={form.provider_type}
                onChange={(_e, v) => updateField('provider_type', v)}
              >
                {PROVIDER_OPTIONS.map((opt) => (
                  <FormSelectOption key={opt.value} value={opt.value} label={opt.label} />
                ))}
              </FormSelect>
            </FormGroup>
            <FormGroup label="Base URL" fieldId="ep-url" style={{ marginTop: '12px' }}>
              <TextInput
                id="ep-url"
                value={form.url}
                onChange={(_e, v) => updateField('url', v)}
                placeholder={form.provider_type === 'gemini' ? '(optional)' : form.provider_type === 'rhoai-maas' ? 'https://rh-ai.apps.example.com/maas-api' : 'https://...'}
              />
              <FormHelperText>
                <HelperText>
                  <HelperTextItem>
                    {form.provider_type === 'gemini' ? 'Optional for Gemini (uses Google API directly)' : form.provider_type === 'rhoai-maas' ? 'MaaS API base URL from your RHOAI cluster' : 'e.g. https://maas.apps.example.com/v1'}
                  </HelperTextItem>
                </HelperText>
              </FormHelperText>
            </FormGroup>
            <FormGroup label="API Key" fieldId="ep-key" style={{ marginTop: '12px' }}>
              <TextInput
                id="ep-key"
                type="password"
                value={form.api_key}
                onChange={(_e, v) => updateField('api_key', v)}
                placeholder={editingId ? '(unchanged)' : form.provider_type === 'rhoai-maas' ? 'oc whoami -t' : 'sk-...'}
              />
              <FormHelperText>
                <HelperText>
                  <HelperTextItem>
                    {form.provider_type === 'rhoai-maas' ? 'OpenShift token (oc whoami -t) or MaaS API key' : 'Leave unchanged to keep existing key'}
                  </HelperTextItem>
                </HelperText>
              </FormHelperText>
            </FormGroup>
            <FormGroup label="Model Name" isRequired fieldId="ep-model" style={{ marginTop: '12px' }}>
              <TextInput
                id="ep-model"
                value={form.model_name}
                onChange={(_e, v) => updateField('model_name', v)}
                placeholder={form.provider_type === 'rhoai-maas' ? 'e.g. ibm-granite-2b-gpu' : 'model name'}
              />
              <FormHelperText>
                <HelperText>
                  <HelperTextItem>
                    {form.provider_type === 'rhoai-maas' ? 'Model slug from MaaS registry (shown in RHOAI dashboard)' : 'e.g. gemini-2.5-flash, meta-llama/Llama-3.1-8B'}
                  </HelperTextItem>
                </HelperText>
              </FormHelperText>
            </FormGroup>
            {isAdmin && (
              <FormGroup fieldId="ep-global" style={{ marginTop: '12px' }}>
                <Switch
                  id="ep-global"
                  label="Share with all users (global)"
                  isChecked={form.is_global}
                  onChange={(_e, v) => updateField('is_global', v)}
                />
              </FormGroup>
            )}
            {editingId && (
              <FormGroup fieldId="ep-enabled" style={{ marginTop: '12px' }}>
                <Switch
                  id="ep-enabled"
                  label="Enabled"
                  isChecked={form.enabled}
                  onChange={(_e, v) => updateField('enabled', v)}
                />
              </FormGroup>
            )}
          </div>
        ) : loading ? (
          <Bullseye><Spinner size="lg" /></Bullseye>
        ) : endpoints.length === 0 ? (
          <EmptyState titleText="No LLM Endpoints" headingLevel="h3" icon={CogIcon}>
            <EmptyStateBody>
              Configure an LLM endpoint to use the GPU Booking Agent with your own model provider.
            </EmptyStateBody>
            <Button variant="primary" onClick={handleAdd} icon={<PlusCircleIcon />}>
              Add Endpoint
            </Button>
          </EmptyState>
        ) : (
          <div>
            <Toolbar>
              <ToolbarContent>
                <ToolbarItem>
                  <Button variant="primary" onClick={handleAdd} icon={<PlusCircleIcon />} size="sm">
                    Add Endpoint
                  </Button>
                </ToolbarItem>
              </ToolbarContent>
            </Toolbar>
            <Table aria-label="LLM Endpoints" variant="compact">
              <Thead>
                <Tr>
                  <Th>Name</Th>
                  <Th>Provider</Th>
                  <Th>Model</Th>
                  <Th>Owner</Th>
                  <Th>Status</Th>
                  <Th>Actions</Th>
                </Tr>
              </Thead>
              <Tbody>
                {endpoints.map((ep) => (
                  <Tr key={ep.id}>
                    <Td dataLabel="Name">
                      {ep.name}
                      {ep.is_global && <Label color="blue" isCompact style={{ marginLeft: '6px' }}>Global</Label>}
                    </Td>
                    <Td dataLabel="Provider">{PROVIDER_OPTIONS.find((o) => o.value === ep.provider_type)?.label || ep.provider_type}</Td>
                    <Td dataLabel="Model">{ep.model_name}</Td>
                    <Td dataLabel="Owner">{ep.owner}</Td>
                    <Td dataLabel="Status">
                      <Label color={ep.enabled ? 'green' : 'grey'} isCompact>
                        {ep.enabled ? 'Enabled' : 'Disabled'}
                      </Label>
                      {testResult?.id === ep.id && (
                        <Label
                          color={testResult.status === 'ok' ? 'green' : 'red'}
                          isCompact
                          style={{ marginLeft: '6px' }}
                        >
                          {testResult.status === 'ok' ? 'Connected' : testResult.message || 'Failed'}
                        </Label>
                      )}
                    </Td>
                    <Td dataLabel="Actions" style={{ whiteSpace: 'nowrap' }}>
                      <Button variant="plain" onClick={() => handleTest(ep.id)} aria-label="Test" title="Test connectivity">
                        <PluggedIcon />
                      </Button>
                      <Button variant="plain" onClick={() => handleEdit(ep)} aria-label="Edit">
                        <PencilAltIcon />
                      </Button>
                      <Button variant="plain" onClick={() => handleDelete(ep.id)} aria-label="Delete" isDanger>
                        <TrashIcon />
                      </Button>
                    </Td>
                  </Tr>
                ))}
              </Tbody>
            </Table>
          </div>
        )}
      </ModalBody>
      <ModalFooter>
        {formOpen ? (
          <>
            <Button variant="primary" onClick={handleSave} isLoading={saving} isDisabled={!form.name || !form.model_name}>
              {editingId ? 'Update' : 'Create'}
            </Button>
            <Button variant="link" onClick={() => setFormOpen(false)}>Cancel</Button>
          </>
        ) : (
          <Button variant="link" onClick={onClose}>Close</Button>
        )}
      </ModalFooter>
    </Modal>
  );
};

export default EndpointSettings;
