import * as React from 'react';
import { Button } from '@patternfly/react-core';
import { Td } from '@patternfly/react-table';
import { Booking, SLOT_TYPE, formatHour } from '../utils/constants';

interface BookingCellProps {
  booking: Booking | undefined;
  resourceType: string;
  unitIdx: number;
  date: string;
  past: boolean;
  activeReservations: Record<string, string>;
  currentUser: string;
  reservingKey: string | null;
  confirmCancelId: string | null;
  onReserve: (resource: string, slotIndex: number, date: string, slotType: string) => void;
  onCancel: (id: string) => void;
  onEdit: (booking: Booking) => void;
  onConfirmCancel: (id: string | null) => void;
}

const BookingCell: React.FC<BookingCellProps> = ({
  booking,
  resourceType,
  unitIdx,
  date,
  past,
  activeReservations,
  currentUser,
  reservingKey,
  confirmCancelId,
  onReserve,
  onCancel,
  onEdit,
  onConfirmCancel,
}) => {
  const cellKey = `${resourceType}-${unitIdx}-${date}-${SLOT_TYPE}`;
  const isReserving = reservingKey === cellKey;

  if (!booking) {
    if (past) {
      return (
        <Td style={{ textAlign: 'center', opacity: 0.7 }}>
          <span style={{ fontSize: '12px', color: 'var(--pf-t--global--text--color--regular)', opacity: 0.7 }}>&mdash;</span>
        </Td>
      );
    }
    return (
      <Td style={{ textAlign: 'center' }}>
        <Button
          variant="primary"
          size="sm"
          onClick={() => onReserve(resourceType, unitIdx, date, SLOT_TYPE)}
          isDisabled={isReserving}
        >
          {isReserving ? '...' : 'Reserve'}
        </Button>
      </Td>
    );
  }

  const confirmButtons = (
    <div style={{ display: 'flex', gap: '4px', marginTop: '4px', justifyContent: 'center' }}>
      <Button variant="danger" size="sm" onClick={() => onCancel(booking.id)}>
        Confirm
      </Button>
      <Button variant="secondary" size="sm" onClick={() => onConfirmCancel(null)}>
        No
      </Button>
    </div>
  );

  const renderActions = () => {
    if (past) {
      return (
        <span style={{ fontSize: '10px', color: 'var(--pf-t--global--text--color--regular)', opacity: 0.7 }}>
          {booking.source === 'consumed' ? '\u26A1 consumed' : activeReservations[booking.user] ? `\uD83D\uDC0D ${activeReservations[booking.user]}` : ''}
        </span>
      );
    }

    if (booking.source === 'consumed') {
      return (
        <div>
          <div style={{ fontSize: '11px', color: 'var(--pf-t--global--color--brand--default)', fontWeight: 500 }}>
            {'\u26A1'} consumed
          </div>
          <Button
            variant="warning"
            size="sm"
            onClick={() => onReserve(resourceType, unitIdx, date, SLOT_TYPE)}
            isDisabled={isReserving}
            style={{ marginTop: '4px' }}
          >
            {isReserving ? '...' : 'Override'}
          </Button>
        </div>
      );
    }

    if (booking.user === currentUser || activeReservations[booking.user]) {
      return (
        <div>
          {activeReservations[booking.user] && (
            <div style={{ fontSize: '11px', color: 'var(--pf-t--global--color--nonstatus--green--default)', fontWeight: 500 }}>
              {'\uD83D\uDC0D'} {activeReservations[booking.user]}
            </div>
          )}
          {confirmCancelId === booking.id ? (
            confirmButtons
          ) : booking.user === currentUser ? (
            <div style={{ display: 'flex', gap: '4px', marginTop: '4px', justifyContent: 'center' }}>
              <Button variant="link" size="sm" onClick={() => onEdit(booking)}>
                Edit
              </Button>
              <Button variant="link" size="sm" isDanger onClick={() => onConfirmCancel(booking.id)}>
                Cancel
              </Button>
            </div>
          ) : null}
        </div>
      );
    }

    if (confirmCancelId === booking.id) {
      return confirmButtons;
    }

    return null;
  };

  return (
    <Td style={{ textAlign: 'center', opacity: past ? 0.7 : 1 }}>
      <div title={booking.description || undefined}>
        <div style={{ fontSize: '13px', fontWeight: 500 }}>
          {booking.user}
        </div>
        {booking.description && (
          <div style={{ fontSize: '10px', color: 'var(--pf-t--global--text--color--regular)', opacity: 0.7, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '100px', margin: '0 auto' }}>
            {booking.description}
          </div>
        )}
        {(booking.startHour !== 0 || booking.endHour !== 24) && (
          <div style={{ fontSize: '10px', color: 'var(--pf-t--global--text--color--regular)', opacity: 0.7 }}>
            {formatHour(booking.startHour)}&mdash;{booking.endHour === 24 ? '00:00' : formatHour(booking.endHour)}{' '}
            (UTC{booking.utcOffset >= 0 ? '+' : ''}{booking.utcOffset})
          </div>
        )}
        {renderActions()}
      </div>
    </Td>
  );
};

export default BookingCell;
