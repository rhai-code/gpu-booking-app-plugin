import * as React from 'react';
import {
  Modal,
  ModalVariant,
  ModalHeader,
  ModalBody,
  ModalFooter,
  Button,
  Form,
  FormGroup,
  TextArea,
  Alert,
  NumberInput,
  FormSelect,
  FormSelectOption,
  Split,
  SplitItem,
} from '@patternfly/react-core';
import {
  Booking,
  GPUResource,
  HOUR_OPTIONS,
  formatHour,
  getUtcOffsetHours,
} from '../utils/constants';

interface BookingModalProps {
  isOpen: boolean;
  startDate: string;
  endDate: string;
  bookings: Booking[];
  editBooking?: Booking;
  gpuResources: GPUResource[];
  onClose: () => void;
  onSubmit: (
    resources: Record<string, number>,
    startDate: string,
    endDate: string,
    description: string,
    startHour: number,
    endHour: number,
    utcOffset: number,
  ) => Promise<void>;
}

const BookingModal: React.FC<BookingModalProps> = ({
  isOpen,
  startDate,
  endDate,
  bookings,
  editBooking,
  gpuResources,
  onClose,
  onSubmit,
}) => {
  const [resources, setResources] = React.useState<Record<string, number>>(
    editBooking ? { [editBooking.resource]: 1 } : {},
  );
  const [start, setStart] = React.useState(editBooking?.date || startDate);
  const [end, setEnd] = React.useState(editBooking?.date || endDate);
  const [startHourLocal, setStartHourLocal] = React.useState(
    editBooking ? editBooking.startHour : 0,
  );
  const [endHourLocal, setEndHourLocal] = React.useState(
    editBooking ? editBooking.endHour : 24,
  );
  const [description, setDescription] = React.useState(editBooking?.description || '');
  const [submitting, setSubmitting] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const utcOffset = getUtcOffsetHours(start);

  const adjustCount = (type: string, delta: number) => {
    setResources((prev) => {
      const current = prev[type] || 0;
      const next = Math.max(0, current + delta);
      if (next === 0) {
        const { [type]: _, ...rest } = prev;
        return rest;
      }
      return { ...prev, [type]: next };
    });
  };

  const getMaxAvailable = (resourceType: string): number => {
    const gpu = gpuResources.find((r) => r.type === resourceType);
    if (!gpu) return 0;
    const [sy, sm, sd] = start.split('-').map(Number);
    const [ey, em, ed] = end.split('-').map(Number);
    const startD = new Date(sy, sm - 1, sd);
    const endD = new Date(ey, em - 1, ed);
    let minAvail = gpu.count;
    for (const d = new Date(startD); d <= endD; d.setDate(d.getDate() + 1)) {
      const y = d.getFullYear();
      const m = String(d.getMonth() + 1).padStart(2, '0');
      const day = String(d.getDate()).padStart(2, '0');
      const dateStr = `${y}-${m}-${day}`;
      const reservedCount = bookings.filter(
        (b) => b.resource === resourceType && b.date === dateStr && b.slotType === 'full' && b.source === 'reserved',
      ).length;
      minAvail = Math.min(minAvail, gpu.count - reservedCount);
    }
    return Math.max(0, minAvail);
  };

  const totalResources = Object.values(resources).reduce((s, c) => s + c, 0);
  const isFullDay = startHourLocal === 0 && endHourLocal === 24;
  const startHourUtc = ((startHourLocal - Math.round(utcOffset)) % 24 + 24) % 24;
  const endHourUtc = endHourLocal === 24 ? 24 : ((endHourLocal - Math.round(utcOffset)) % 24 + 24) % 24;

  const gpuEquivMap: Record<string, number> = {};
  for (const r of gpuResources) gpuEquivMap[r.type] = r.gpuEquivalent;

  const gpuEquivTotal = Object.entries(resources).reduce(
    (sum, [type, count]) => sum + count * (gpuEquivMap[type] || 0),
    0,
  );

  const handleSubmit = async () => {
    if (totalResources === 0) return;
    setSubmitting(true);
    setError(null);
    try {
      await onSubmit(resources, start, end, description, startHourLocal, endHourLocal, Math.round(utcOffset));
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to create bookings');
      setSubmitting(false);
    }
  };

  return (
    <Modal
      variant={ModalVariant.medium}
      isOpen={isOpen}
      onClose={onClose}
    >
      <ModalHeader title={editBooking ? 'Edit Reservation' : 'Book GPU Resources'} />
      <ModalBody>
        <Form>
          <FormGroup label="Date Range" fieldId="date-range">
            <Split hasGutter>
              <SplitItem isFilled>
                <input
                  type="date"
                  value={start}
                  onChange={(e) => setStart(e.target.value)}
                  style={{ width: '100%', padding: '6px 12px', border: '1px solid var(--pf-t--global--border--color--default)', borderRadius: '4px' }}
                />
              </SplitItem>
              <SplitItem style={{ display: 'flex', alignItems: 'center' }}>to</SplitItem>
              <SplitItem isFilled>
                <input
                  type="date"
                  value={end}
                  onChange={(e) => setEnd(e.target.value)}
                  style={{ width: '100%', padding: '6px 12px', border: '1px solid var(--pf-t--global--border--color--default)', borderRadius: '4px' }}
                />
              </SplitItem>
            </Split>
          </FormGroup>

          <FormGroup
            label={`Hours (local time, UTC${utcOffset >= 0 ? '+' : ''}${utcOffset})`}
            fieldId="hours"
          >
            <Split hasGutter>
              <SplitItem isFilled>
                <FormSelect
                  value={startHourLocal}
                  onChange={(_e, val) => setStartHourLocal(Number(val))}
                  aria-label="Start hour"
                >
                  {HOUR_OPTIONS.filter((h) => h < 24).map((h) => (
                    <FormSelectOption key={h} value={h} label={formatHour(h)} />
                  ))}
                </FormSelect>
              </SplitItem>
              <SplitItem style={{ display: 'flex', alignItems: 'center' }}>to</SplitItem>
              <SplitItem isFilled>
                <FormSelect
                  value={endHourLocal}
                  onChange={(_e, val) => setEndHourLocal(Number(val))}
                  aria-label="End hour"
                >
                  {HOUR_OPTIONS.filter((h) => h > startHourLocal).map((h) => (
                    <FormSelectOption key={h} value={h} label={formatHour(h)} />
                  ))}
                </FormSelect>
              </SplitItem>
            </Split>
            {!isFullDay && (
              <div style={{ fontSize: '12px', color: 'var(--pf-t--global--text--color--regular)', opacity: 0.7, marginTop: '4px' }}>
                UTC: {formatHour(startHourUtc)} &mdash; {endHourUtc === 24 ? '00:00 +1d' : formatHour(endHourUtc)}
              </div>
            )}
          </FormGroup>

          <FormGroup label="Resources" fieldId="resources">
            {gpuResources.map((r) => {
              const count = resources[r.type] || 0;
              const maxAvail = getMaxAvailable(r.type);
              const equiv = r.gpuEquivalent;

              return (
                <div
                  key={r.type}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    padding: '12px',
                    marginBottom: '8px',
                    borderRadius: '8px',
                    border: `1px solid ${count > 0 ? 'var(--pf-t--global--color--brand--default)' : 'var(--pf-t--global--border--color--default)'}`,
                    backgroundColor: count > 0 ? 'var(--pf-t--global--background--color--primary--default)' : undefined,
                  }}
                >
                  <div>
                    <div style={{ fontSize: '14px', fontWeight: 500 }}>{r.name}</div>
                    <div style={{ fontSize: '12px', color: 'var(--pf-t--global--text--color--regular)', opacity: 0.7 }}>
                      {maxAvail} of {r.count} available
                      {equiv < 1 && ` \u00B7 ${equiv} GPU equiv each`}
                    </div>
                  </div>
                  <NumberInput
                    value={count}
                    min={0}
                    max={maxAvail}
                    onMinus={() => adjustCount(r.type, -1)}
                    onPlus={() => adjustCount(r.type, 1)}
                    onChange={(e) => {
                      const val = Number((e.target as HTMLInputElement).value);
                      if (!isNaN(val)) {
                        const clamped = Math.max(0, Math.min(maxAvail, val));
                        setResources((prev) => {
                          if (clamped === 0) {
                            const { [r.type]: _, ...rest } = prev;
                            return rest;
                          }
                          return { ...prev, [r.type]: clamped };
                        });
                      }
                    }}
                    minusBtnAriaLabel="Decrease"
                    plusBtnAriaLabel="Increase"
                    inputAriaLabel={`${r.name} count`}
                    widthChars={2}
                  />
                </div>
              );
            })}
            {gpuEquivTotal > 0 && (
              <div style={{ fontSize: '12px', color: 'var(--pf-t--global--text--color--regular)', opacity: 0.7, textAlign: 'right', marginTop: '4px' }}>
                Total: {gpuEquivTotal % 1 === 0 ? gpuEquivTotal : gpuEquivTotal.toFixed(2)} GPU equivalents
              </div>
            )}
          </FormGroup>

          <FormGroup label={`Description (${160 - description.length} chars remaining)`} fieldId="description">
            <TextArea
              id="description"
              value={description}
              onChange={(_e, val) => setDescription(val.slice(0, 160))}
              maxLength={160}
              rows={2}
              placeholder="What will you use these GPUs for?"
            />
          </FormGroup>

          {error && (
            <Alert variant="danger" title="Error" isInline>
              {error}
            </Alert>
          )}
        </Form>
      </ModalBody>
      <ModalFooter>
        <Button
          variant="primary"
          onClick={handleSubmit}
          isDisabled={submitting || totalResources === 0}
          isLoading={submitting}
        >
          {editBooking ? 'Save Changes' : `Book ${totalResources} resource${totalResources !== 1 ? 's' : ''}`}
        </Button>
        <Button variant="link" onClick={onClose}>
          Cancel
        </Button>
      </ModalFooter>
    </Modal>
  );
};

export default BookingModal;
