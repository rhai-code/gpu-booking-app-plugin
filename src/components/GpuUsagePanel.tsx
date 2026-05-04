import * as React from 'react';
import { Card, CardBody, Title, Tooltip } from '@patternfly/react-core';
import {
  Booking,
  GPUResource,
  RESOURCE_COLORS,
  FREE_COLOR,
  CONSUMED_COLOR,
  RESERVED_COLOR,
} from '../utils/constants';

interface GpuUsagePanelProps {
  bookings: Booking[];
  resources: GPUResource[];
  selectedDate?: string;
}

function localDateStr(): string {
  const n = new Date();
  return `${n.getFullYear()}-${String(n.getMonth() + 1).padStart(2, '0')}-${String(n.getDate()).padStart(2, '0')}`;
}

function utcDateStr(): string {
  const n = new Date();
  return `${n.getUTCFullYear()}-${String(n.getUTCMonth() + 1).padStart(2, '0')}-${String(n.getUTCDate()).padStart(2, '0')}`;
}

const GpuUsagePanel: React.FC<GpuUsagePanelProps> = ({ bookings, resources, selectedDate }) => {
  const today = selectedDate || localDateStr();
  const utcToday = utcDateStr();
  const localToday = localDateStr();
  const showAdjacentUtcDate = today === localToday && utcToday !== localToday;
  const displayDates = showAdjacentUtcDate ? [today, utcToday] : [today];

  const usageData = React.useMemo(() => {
    return resources.map((r) => {
      const dayBookings = bookings.filter(
        (b) => b.resource === r.type && displayDates.includes(b.date),
      );
      const bookedUnits = new Set<number>();
      const reservedUnits = new Set<number>();
      const consumedUnits = new Set<number>();
      for (const b of dayBookings) {
        bookedUnits.add(b.slotIndex);
        if (b.source === 'consumed') consumedUnits.add(b.slotIndex);
        else reservedUnits.add(b.slotIndex);
      }
      return {
        resource: r,
        reservedCount: reservedUnits.size,
        consumedCount: consumedUnits.size,
        totalBooked: bookedUnits.size,
        totalSlots: r.count,
      };
    });
  }, [bookings, resources, today, showAdjacentUtcDate]);

  const totalBooked = usageData.reduce((s, u) => s + u.totalBooked, 0);
  const totalSlots = usageData.reduce((s, u) => s + u.totalSlots, 0);

  const formatDisplayDate = (d: string) => {
    const date = new Date(d + 'T00:00:00');
    return date.toLocaleDateString('en-GB', {
      weekday: 'long',
      day: 'numeric',
      month: 'long',
      year: 'numeric',
    });
  };

  return (
    <Card style={{ marginBottom: '16px' }}>
      <CardBody>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '16px' }}>
          <div>
            <Title headingLevel="h2" size="lg">
              GPU Usage Overview
            </Title>
            <div style={{ fontSize: '12px', color: 'var(--pf-t--global--text--color--regular)', opacity: 0.7, marginTop: '4px' }}>
              {formatDisplayDate(today)}
              {showAdjacentUtcDate && <span> + {formatDisplayDate(utcToday)} (UTC)</span>}
              {' '}&mdash; {totalBooked} of {totalSlots} slots booked
            </div>
          </div>
          <div style={{ display: 'flex', gap: '16px', fontSize: '12px', color: 'var(--pf-t--global--text--color--regular)', opacity: 0.7 }}>
            <span><span style={{ color: CONSUMED_COLOR }}>&#9679;</span> Consumed</span>
            <span><span style={{ color: RESERVED_COLOR }}>&#9679;</span> Reserved</span>
            <span><span style={{ color: FREE_COLOR }}>&#9679;</span> Free</span>
          </div>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          {usageData.map((usage) => {
            const { resource, totalSlots } = usage;
            return (
              <div key={resource.type}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <span
                      style={{
                        width: '10px',
                        height: '10px',
                        borderRadius: '50%',
                        display: 'inline-block',
                        backgroundColor: RESOURCE_COLORS[resource.type] || '#888',
                      }}
                    />
                    <span style={{ fontSize: '14px', fontWeight: 600 }}>{resource.name}</span>
                    <span style={{ fontSize: '12px', color: 'var(--pf-t--global--text--color--regular)', opacity: 0.7 }}>
                      {resource.count} units
                    </span>
                  </div>
                  <span style={{ fontSize: '12px', color: 'var(--pf-t--global--text--color--regular)', opacity: 0.7 }}>
                    {usage.totalBooked} / {totalSlots} booked
                  </span>
                </div>

                <div style={{ display: 'flex', height: '32px', borderRadius: '4px', overflow: 'hidden', backgroundColor: 'var(--pf-t--global--background--color--secondary--default)' }}>
                  {Array.from({ length: totalSlots }, (_, unitIdx) => {
                    const unitBookings = bookings.filter(
                      (b) => b.resource === resource.type && displayDates.includes(b.date) && b.slotIndex === unitIdx,
                    );
                    const hasReserved = unitBookings.some((b) => b.source === 'reserved');
                    const hasConsumed = unitBookings.some((b) => b.source === 'consumed');
                    const color = hasReserved ? RESERVED_COLOR : hasConsumed ? CONSUMED_COLOR : FREE_COLOR;
                    const userName = unitBookings.length > 0 ? unitBookings[0].user : '';

                    return (
                      <Tooltip
                        key={unitIdx}
                        content={
                          <div>
                            Unit {unitIdx + 1}: {unitBookings.length > 0 ? `booked` : 'free'}
                            {unitBookings.map((b) => (
                              <div key={b.id}>{b.user} ({b.source})</div>
                            ))}
                          </div>
                        }
                      >
                        <div
                          className="gpu-usage-bar-segment"
                          style={{
                            width: `${100 / totalSlots}%`,
                            height: '100%',
                            backgroundColor: color,
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'center',
                            fontSize: '10px',
                            fontWeight: 'bold',
                            color: 'white',
                            overflow: 'hidden',
                          }}
                        >
                          {userName && <span className="gpu-usage-bar-label">{userName}</span>}
                        </div>
                      </Tooltip>
                    );
                  })}
                </div>

                <div style={{ display: 'flex', gap: '4px', marginTop: '6px' }}>
                  {Array.from({ length: resource.count }, (_, unitIdx) => {
                    const unitBookings = bookings.filter(
                      (b) =>
                        b.resource === resource.type &&
                        displayDates.includes(b.date) &&
                        b.slotIndex === unitIdx,
                    );
                    const hasReserved = unitBookings.some((b) => b.source === 'reserved');
                    const hasConsumed = unitBookings.some((b) => b.source === 'consumed');
                    let bgColor = FREE_COLOR;
                    let opacity = 0.4;
                    if (hasReserved) {
                      bgColor = RESERVED_COLOR;
                      opacity = 1;
                    } else if (hasConsumed) {
                      bgColor = CONSUMED_COLOR;
                      opacity = 1;
                    }

                    return (
                      <Tooltip
                        key={unitIdx}
                        content={
                          <div>
                            Unit {unitIdx + 1}: {unitBookings.length > 0 ? `${unitBookings.length} slot(s) booked` : 'free'}
                            {unitBookings.map((b) => (
                              <div key={b.id}>
                                {b.user} ({b.slotType}) {b.source === 'consumed' ? '\u26A1' : ''}
                              </div>
                            ))}
                          </div>
                        }
                      >
                        <div
                          style={{
                            flex: 1,
                            height: '8px',
                            borderRadius: '2px',
                            backgroundColor: bgColor,
                            opacity,
                          }}
                        />
                      </Tooltip>
                    );
                  })}
                </div>
              </div>
            );
          })}
        </div>
      </CardBody>
    </Card>
  );
};

export default GpuUsagePanel;
