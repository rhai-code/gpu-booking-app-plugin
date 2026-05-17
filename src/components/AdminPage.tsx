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
  TextInput,
  Switch,
  Card,
  CardBody,
  Label,
  Modal,
  ModalVariant,
  ModalHeader,
  ModalBody,
  ModalFooter,
  FileUpload,
  Pagination,
  Split,
  SplitItem,
} from '@patternfly/react-core';
import { SyncIcon, DownloadIcon, UploadIcon, TrashIcon, SearchIcon } from '@patternfly/react-icons';
import {
  Table,
  Thead,
  Tr,
  Th,
  Tbody,
  Td,
  ThProps,
} from '@patternfly/react-table';
import { useAuth } from '../utils/AuthContext';
import {
  adminGetBookings,
  adminDeleteBooking,
  adminDeleteAllBookings,
  adminDeleteOldBookings,
  adminToggleReservationSync,
  adminExportDatabase,
  adminImportDatabase,
  adminTriggerDiscovery,
  AdminResponse,
} from '../utils/api';
import { GPUResource, FALLBACK_GPU_RESOURCES, todayStr } from '../utils/constants';
import ResourceSelector from './ResourceSelector';

type SortKey = 'id' | 'user' | 'resource' | 'slotIndex' | 'date' | 'source' | 'createdAt';

const AdminPage: React.FC = () => {
  const { isAdmin, loading: authLoading } = useAuth();
  const [data, setData] = React.useState<AdminResponse | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [filter, setFilter] = React.useState('');
  const [sortKey, setSortKey] = React.useState<SortKey>('date');
  const [sortDir, setSortDir] = React.useState<'asc' | 'desc'>('desc');
  const [confirmDeleteId, setConfirmDeleteId] = React.useState<string | null>(null);
  const [showDeleteAll, setShowDeleteAll] = React.useState(false);
  const [showDeleteOld, setShowDeleteOld] = React.useState(false);
  const [importFile, setImportFile] = React.useState<File | null>(null);
  const [importFilename, setImportFilename] = React.useState('');
  const [discovering, setDiscovering] = React.useState(false);
  const [gpuResources, setGpuResources] = React.useState<GPUResource[]>(FALLBACK_GPU_RESOURCES);
  const [selectedResources, setSelectedResources] = React.useState<string[]>([]);
  const [sourceFilter, setSourceFilter] = React.useState<'all' | 'reserved' | 'consumed'>('all');
  const [page, setPage] = React.useState(1);
  const [perPage, setPerPage] = React.useState(100);

  // Only pass a single resource to the server filter when exactly one is selected
  const serverResource = selectedResources.length === 1 ? selectedResources[0] : '';
  const serverSource = sourceFilter !== 'all' ? sourceFilter : '';

  const fetchData = React.useCallback(async () => {
    try {
      const offset = (page - 1) * perPage;
      const result = await adminGetBookings({
        limit: perPage,
        offset,
        source: serverSource || undefined,
        resource: serverResource || undefined,
        search: filter || undefined,
      });
      setData(result);
      if (result.config?.resources?.length > 0) {
        setGpuResources(result.config.resources);
        setSelectedResources((prev) => prev.length === 0 ? result.config.resources.map((r) => r.type) : prev);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load admin data');
    }
    setLoading(false);
  }, [page, perPage, serverSource, serverResource, filter]);

  React.useEffect(() => {
    if (!authLoading && isAdmin) {
      fetchData();
      const interval = setInterval(fetchData, 30000);
      return () => clearInterval(interval);
    }
  }, [authLoading, isAdmin, fetchData]);

  const handleDelete = async (id: string) => {
    try {
      await adminDeleteBooking(id);
      setConfirmDeleteId(null);
      await fetchData();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed');
    }
  };

  const handleDeleteAll = async () => {
    try {
      await adminDeleteAllBookings();
      setShowDeleteAll(false);
      await fetchData();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete all failed');
    }
  };

  const handleDeleteOld = async () => {
    try {
      await adminDeleteOldBookings(todayStr());
      setShowDeleteOld(false);
      await fetchData();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete old bookings failed');
    }
  };

  const handleToggleSync = async (enabled: boolean) => {
    try {
      const result = await adminToggleReservationSync(enabled);
      setData((prev) => prev ? { ...prev, reservationSyncEnabled: result.reservationSyncEnabled } : prev);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Toggle failed');
    }
  };

  const handleDiscover = async () => {
    setDiscovering(true);
    setError(null);
    try {
      const result = await adminTriggerDiscovery();
      if (result.resources?.length > 0) {
        setGpuResources(result.resources);
        setSelectedResources(result.resources.map((r) => r.type));
      }
      await fetchData();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'GPU discovery failed');
    }
    setDiscovering(false);
  };

  const handleImport = async () => {
    if (!importFile) return;
    try {
      await adminImportDatabase(importFile);
      setImportFile(null);
      setImportFilename('');
      await fetchData();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Import failed');
    }
  };

  if (authLoading) {
    return (
      <Bullseye>
        <Spinner size="xl" />
      </Bullseye>
    );
  }

  if (!isAdmin) {
    return (
      <>
        <PageSection>
          <Alert variant="danger" title="Access Denied" isInline>
            You do not have admin privileges. Contact your cluster administrator.
          </Alert>
        </PageSection>
      </>
    );
  }

  if (loading) {
    return (
      <Bullseye>
        <Spinner size="xl" />
      </Bullseye>
    );
  }

  const bookings = data?.bookings || [];

  // Client-side filter only for multi-resource selection (single resource is server-side)
  const filtered = bookings.filter((b) => {
    if (selectedResources.length > 1 && !selectedResources.includes(b.resource)) return false;
    return true;
  });

  // Sort
  const sorted = [...filtered].sort((a, b) => {
    const aVal = a[sortKey] as string | number;
    const bVal = b[sortKey] as string | number;
    const cmp = aVal < bVal ? -1 : aVal > bVal ? 1 : 0;
    return sortDir === 'asc' ? cmp : -cmp;
  });

  const getSortParams = (key: SortKey): ThProps['sort'] => ({
    sortBy: {
      index: ['id', 'user', 'resource', 'slotIndex', 'date', 'source', 'createdAt'].indexOf(sortKey),
      direction: sortDir,
    },
    onSort: (_e, _index, direction) => {
      setSortKey(key);
      setSortDir(direction);
    },
    columnIndex: ['id', 'user', 'resource', 'slotIndex', 'date', 'source', 'createdAt'].indexOf(key),
  });

  return (
    <>
      <Helmet>
        <title>GPU Booking - Administration</title>
      </Helmet>
      <>
        <PageSection>
          <Title headingLevel="h1" size="2xl">
            GPU Booking Administration
          </Title>
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

          {/* Admin controls */}
          <Card style={{ marginBottom: '16px' }}>
            <CardBody>
              <Split hasGutter>
                <SplitItem>
                  <Switch
                    id="sync-toggle"
                    label={data?.reservationSyncEnabled ? 'Reservation Sync ON' : 'Reservation Sync OFF'}
                    isChecked={data?.reservationSyncEnabled || false}
                    onChange={(_e, checked) => handleToggleSync(checked)}
                  />
                </SplitItem>
                <SplitItem>
                  <Button
                    variant="secondary"
                    icon={<SearchIcon />}
                    onClick={handleDiscover}
                    isLoading={discovering}
                    isDisabled={discovering}
                  >
                    Discover GPUs
                  </Button>
                </SplitItem>
                <SplitItem>
                  <Button variant="secondary" icon={<DownloadIcon />} onClick={() => adminExportDatabase()}>
                    Export DB
                  </Button>
                </SplitItem>
                <SplitItem>
                  <FileUpload
                    id="import-file"
                    type="dataURL"
                    filename={importFilename}
                    onFileInputChange={(_e, file) => {
                      setImportFile(file);
                      setImportFilename(file.name);
                    }}
                    onClearClick={() => { setImportFile(null); setImportFilename(''); }}
                    browseButtonText="Choose DB"
                    style={{ maxWidth: '300px' }}
                  />
                </SplitItem>
                {importFile && (
                  <SplitItem>
                    <Button variant="primary" icon={<UploadIcon />} onClick={handleImport}>
                      Import
                    </Button>
                  </SplitItem>
                )}
                <SplitItem isFilled />
                <SplitItem>
                  <Button variant="warning" icon={<TrashIcon />} onClick={() => setShowDeleteOld(true)}>
                    Delete All Old
                  </Button>
                </SplitItem>
                <SplitItem>
                  <Button variant="danger" icon={<TrashIcon />} onClick={() => setShowDeleteAll(true)}>
                    Delete All
                  </Button>
                </SplitItem>
              </Split>
            </CardBody>
          </Card>

          {/* Resource filter */}
          <div style={{ marginBottom: '16px' }}>
            <ResourceSelector
              resources={gpuResources}
              selectedResources={selectedResources}
              onSelectionChange={(res) => { setSelectedResources(res); setPage(1); }}
            />
          </div>

          {/* Source filter */}
          <div style={{ marginBottom: '16px', display: 'flex', alignItems: 'center', gap: '8px' }}>
            <span style={{ fontSize: '14px', color: 'var(--pf-t--global--text--color--regular)', opacity: 0.7, marginRight: '4px' }}>Source:</span>
            {(['all', 'reserved', 'consumed'] as const).map((s) => (
              <Button
                key={s}
                variant={sourceFilter === s ? 'primary' : 'secondary'}
                size="sm"
                onClick={() => { setSourceFilter(s); setPage(1); }}
                style={undefined}
              >
                {s === 'all' ? 'All' : s === 'reserved' ? 'Reserved' : 'Consumed'}
              </Button>
            ))}
          </div>

          {/* Toolbar */}
          <Toolbar>
            <ToolbarContent>
              <ToolbarItem>
                <TextInput
                  type="search"
                  aria-label="Filter bookings"
                  placeholder="Filter by user, date, resource..."
                  value={filter}
                  onChange={(_e, val) => { setFilter(val); setPage(1); }}
                  style={{ minWidth: '300px' }}
                />
              </ToolbarItem>
              <ToolbarItem>
                <Button variant="plain" onClick={() => { setLoading(true); fetchData(); }} icon={<SyncIcon />}>
                  Refresh
                </Button>
              </ToolbarItem>
              <ToolbarItem>
                <span style={{ fontSize: '14px', color: 'var(--pf-t--global--text--color--regular)', opacity: 0.7 }}>
                  {sorted.length} of {data?.total ?? 0} bookings
                </span>
              </ToolbarItem>
            </ToolbarContent>
          </Toolbar>

          {/* Bookings table */}
          <Table aria-label="Admin bookings table" variant="compact">
            <Thead>
              <Tr>
                <Th sort={getSortParams('id')}>ID</Th>
                <Th sort={getSortParams('user')}>User</Th>
                <Th sort={getSortParams('resource')}>Resource</Th>
                <Th sort={getSortParams('slotIndex')}>Slot</Th>
                <Th sort={getSortParams('date')}>Date</Th>
                <Th sort={getSortParams('source')}>Source</Th>
                <Th sort={getSortParams('createdAt')}>Created</Th>
                <Th>Actions</Th>
              </Tr>
            </Thead>
            <Tbody>
              {sorted.map((b) => (
                <Tr key={b.id}>
                  <Td style={{ fontSize: '12px', maxWidth: '150px', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                    {b.id}
                  </Td>
                  <Td>{b.user}</Td>
                  <Td style={{ fontSize: '12px' }}>{b.resource}</Td>
                  <Td>{b.slotIndex}</Td>
                  <Td>{b.date}</Td>
                  <Td>
                    <Label color={b.source === 'consumed' ? 'green' : 'red'} isCompact>
                      {b.source}
                    </Label>
                  </Td>
                  <Td style={{ fontSize: '12px' }}>{b.createdAt}</Td>
                  <Td>
                    {confirmDeleteId === b.id ? (
                      <Split hasGutter>
                        <SplitItem>
                          <Button variant="danger" size="sm" onClick={() => handleDelete(b.id)}>
                            Confirm
                          </Button>
                        </SplitItem>
                        <SplitItem>
                          <Button variant="secondary" size="sm" onClick={() => setConfirmDeleteId(null)}>
                            Cancel
                          </Button>
                        </SplitItem>
                      </Split>
                    ) : (
                      <Button variant="link" isDanger size="sm" onClick={() => setConfirmDeleteId(b.id)}>
                        Delete
                      </Button>
                    )}
                  </Td>
                </Tr>
              ))}
            </Tbody>
          </Table>

          <Pagination
            itemCount={data?.total ?? 0}
            perPage={perPage}
            page={page}
            onSetPage={(_e, p) => setPage(p)}
            onPerPageSelect={(_e, pp) => { setPerPage(pp); setPage(1); }}
            perPageOptions={[
              { title: '20', value: 20 },
              { title: '50', value: 50 },
              { title: '100', value: 100 },
              { title: '200', value: 200 },
            ]}
            style={{ marginTop: '16px' }}
          />
        </PageSection>

        {/* Delete All confirmation modal */}
        <Modal
          variant={ModalVariant.small}
          isOpen={showDeleteAll}
          onClose={() => setShowDeleteAll(false)}
        >
          <ModalHeader title="Delete All Bookings" />
          <ModalBody>
            Are you sure you want to delete all bookings? This action cannot be undone.
          </ModalBody>
          <ModalFooter>
            <Button variant="danger" onClick={handleDeleteAll}>
              Delete All
            </Button>
            <Button variant="link" onClick={() => setShowDeleteAll(false)}>
              Cancel
            </Button>
          </ModalFooter>
        </Modal>

        {/* Delete All Old confirmation modal */}
        <Modal
          variant={ModalVariant.small}
          isOpen={showDeleteOld}
          onClose={() => setShowDeleteOld(false)}
        >
          <ModalHeader title="Delete All Old Bookings" />
          <ModalBody>
            Are you sure you want to delete all bookings before today's date? This action cannot be undone.
          </ModalBody>
          <ModalFooter>
            <Button variant="warning" onClick={handleDeleteOld}>
              Delete All Old
            </Button>
            <Button variant="link" onClick={() => setShowDeleteOld(false)}>
              Cancel
            </Button>
          </ModalFooter>
        </Modal>
      </>
    </>
  );
};

export default AdminPage;
