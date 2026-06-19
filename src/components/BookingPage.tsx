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
  EmptyState,
  EmptyStateBody,
} from '@patternfly/react-core';
import { SyncIcon, UserIcon, OutlinedQuestionCircleIcon } from '@patternfly/react-icons';
import { Link } from 'react-router-dom';
import { useAuth } from '../utils/AuthContext';
import {
  createBooking as apiCreateBooking,
  createBulkBooking,
  cancelBooking,
  bulkCancelBookings,
} from '../utils/api';
import {
  Booking,
  todayStr,
  getUtcOffsetHours,
} from '../utils/constants';
import { useBookings, useConfig, useClock, usePreemptedWorkloads } from '../utils/hooks';
import ResourceSelector from './ResourceSelector';
import CalendarGrid from './CalendarGrid';
import GpuUsagePanel from './GpuUsagePanel';
import BookingGrid from './BookingGrid';
import BookingModal from './BookingModal';
import PreemptionBanner from './PreemptionBanner';

const BookingPage: React.FC = () => {
  useAuth();
  const { bookings, activeReservations, currentUser, loading: bookingsLoading, fetchBookings } = useBookings();
  const { gpuResources, bookingWindowDays } = useConfig();
  const { preemptedWorkloads } = usePreemptedWorkloads();
  const utcNow = useClock();

  const [loading, setLoading] = React.useState(true);
  const [reservingKey, setReservingKey] = React.useState<string | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [selectedResources, setSelectedResources] = React.useState<string[]>([]);
  const [confirmCancelId, setConfirmCancelId] = React.useState<string | null>(null);
  const [showBookingModal, setShowBookingModal] = React.useState(false);
  const [editBooking, setEditBooking] = React.useState<Booking | null>(null);
  const [contextMenu, setContextMenu] = React.useState<{ x: number; y: number } | null>(null);
  const [showingMyBookings, setShowingMyBookings] = React.useState(false);
  const [confirmCancelAll, setConfirmCancelAll] = React.useState(false);
  const [cancellingAll, setCancellingAll] = React.useState(false);

  const now = new Date();
  const [viewYear, setViewYear] = React.useState(now.getFullYear());
  const [viewMonth, setViewMonth] = React.useState(now.getMonth());
  const [selectedDates, setSelectedDates] = React.useState<string[]>([todayStr()]);

  // Initialize selected resource when config loads
  React.useEffect(() => {
    if (gpuResources.length > 0 && selectedResources.length === 0) {
      setSelectedResources([gpuResources[0].type]);
    }
  }, [gpuResources]);

  // Sync loading state from bookings hook
  React.useEffect(() => {
    if (!bookingsLoading) setLoading(false);
  }, [bookingsLoading]);

  const selectedResourceObjects = gpuResources.filter((r) => selectedResources.includes(r.type));
  const gridDates = React.useMemo(() => [...selectedDates].sort(), [selectedDates]);

  // When resource selection changes while "My Bookings" is active, re-filter dates
  React.useEffect(() => {
    if (!showingMyBookings || !currentUser) return;
    const myBookings = bookings.filter(
      (b) => b.user === currentUser && b.source === 'reserved' && selectedResources.includes(b.resource),
    );
    const myDates = Array.from(new Set(myBookings.map((b) => b.date))).sort();
    if (myDates.length === 0) {
      setShowingMyBookings(false);
      setSelectedDates([todayStr()]);
      return;
    }
    setSelectedDates(myDates);
  }, [selectedResources, showingMyBookings, currentUser, bookings]);

  React.useEffect(() => {
    if (!contextMenu) return;
    const close = () => setContextMenu(null);
    window.addEventListener('click', close);
    return () => window.removeEventListener('click', close);
  }, [contextMenu]);

  // Navigation limits
  const maxDate = new Date();
  maxDate.setDate(maxDate.getDate() + bookingWindowDays);
  const earliestBooking = bookings.length > 0
    ? bookings.reduce((min, b) => (b.date < min ? b.date : min), bookings[0].date)
    : null;
  const earliestDate = earliestBooking
    ? (() => { const [ey, em] = earliestBooking.split('-').map(Number); return new Date(ey, em - 1, 1); })()
    : now;
  const canGoBack = viewYear > earliestDate.getFullYear() || (viewYear === earliestDate.getFullYear() && viewMonth > earliestDate.getMonth());
  const canGoForward = viewYear < maxDate.getFullYear() || (viewYear === maxDate.getFullYear() && viewMonth < maxDate.getMonth());

  const handleReserve = async (resource: string, slotIndex: number, date: string, slotType: string) => {
    const key = `${resource}-${slotIndex}-${date}-${slotType}`;
    setReservingKey(key);
    setError(null);
    try {
      await apiCreateBooking({ resource, slotIndex, date, slotType, startHour: 0, endHour: 24, utcOffset: Math.round(getUtcOffsetHours(date)) });
      await fetchBookings();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to reserve');
    }
    setReservingKey(null);
  };

  const handleCancel = async (id: string) => {
    setError(null);
    try {
      await cancelBooking(id);
      setConfirmCancelId(null);
      await fetchBookings();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to cancel');
    }
  };

  const handleBulkBooking = async (
    resources: Record<string, number>,
    startDate: string,
    endDate: string,
    description: string,
    startHour: number,
    endHour: number,
    utcOffset: number,
  ) => {
    if (editBooking) {
      await cancelBooking(editBooking.id);
    }
    await createBulkBooking({ resources, startDate, endDate, description, startHour, endHour, utcOffset });
    setShowBookingModal(false);
    setEditBooking(null);
    await fetchBookings();
  };

  const navigateMonth = (delta: number) => {
    let newMonth = viewMonth + delta;
    let newYear = viewYear;
    if (newMonth > 11) { newMonth = 0; newYear++; }
    else if (newMonth < 0) { newMonth = 11; newYear--; }
    setViewMonth(newMonth);
    setViewYear(newYear);
    setSelectedDates([]);
  };

  const handleShowMyBookings = () => {
    if (!currentUser) return;
    if (showingMyBookings) {
      setShowingMyBookings(false);
      setSelectedResources([gpuResources[0].type]);
      setViewMonth(now.getMonth());
      setViewYear(now.getFullYear());
      setSelectedDates([todayStr()]);
      return;
    }
    const myBookings = bookings.filter((b) => b.user === currentUser && b.source === 'reserved');
    const myDates = Array.from(new Set(myBookings.map((b) => b.date))).sort();
    if (myDates.length === 0) return;
    const myResourceTypes = Array.from(new Set(myBookings.map((b) => b.resource)));
    setSelectedResources(myResourceTypes);
    const [fy, fm] = myDates[0].split('-').map(Number);
    setViewYear(fy);
    setViewMonth(fm - 1);
    setSelectedDates(myDates);
    setShowingMyBookings(true);
  };

  const handleCancelAll = async () => {
    if (!currentUser) return;
    const ids = bookings
      .filter((b) => b.user === currentUser && b.source === 'reserved' && selectedResources.includes(b.resource))
      .map((b) => b.id);
    if (ids.length === 0) return;
    setCancellingAll(true);
    setError(null);
    try {
      await bulkCancelBookings(ids);
      setConfirmCancelAll(false);
      await fetchBookings();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to cancel bookings');
    }
    setCancellingAll(false);
  };

  const cancelAllCount = React.useMemo(() => {
    if (!currentUser) return 0;
    return bookings.filter(
      (b) => b.user === currentUser && b.source === 'reserved' && selectedResources.includes(b.resource),
    ).length;
  }, [bookings, currentUser, selectedResources]);

  const handleContextMenu = (dateStr: string, e: React.MouseEvent) => {
    e.preventDefault();
    if (!selectedDates.includes(dateStr)) {
      setSelectedDates([dateStr]);
    }
    setContextMenu({ x: e.clientX, y: e.clientY });
  };

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
        <title>GPU Booking</title>
      </Helmet>
      <>
        <PageSection style={{ paddingBottom: '24px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
            <div>
              <Title headingLevel="h1" size="2xl">
                GPU Resource Booking
              </Title>
              <div style={{ color: 'var(--pf-t--global--text--color--regular)', opacity: 0.7, marginTop: '8px' }}>
                Reserve GPU resources
                {gpuResources.length > 0 && ` — ${gpuResources[0].name.replace(/ Full GPU$/, '')}`}
                {gpuResources.length > 1 && ' with MIG partitioning'}
              </div>
              <div style={{ fontFamily: 'monospace', color: 'var(--pf-t--global--text--color--regular)', opacity: 0.7, fontSize: '13px', marginTop: '4px' }}>
                {utcNow}
              </div>
            </div>
            <Toolbar>
              <ToolbarContent>
                {currentUser && (
                  <ToolbarItem>
                    <Button
                      variant={showingMyBookings ? 'primary' : 'secondary'}
                      onClick={handleShowMyBookings}
                      icon={<UserIcon />}
                    >
                      My Bookings
                    </Button>
                  </ToolbarItem>
                )}
                {showingMyBookings && cancelAllCount > 0 && (
                  <ToolbarItem>
                    {confirmCancelAll ? (
                      <>
                        <span style={{ marginRight: '8px', fontSize: '14px' }}>
                          Cancel {cancelAllCount} booking{cancelAllCount !== 1 ? 's' : ''}?
                        </span>
                        <Button
                          variant="danger"
                          onClick={handleCancelAll}
                          isDisabled={cancellingAll}
                          isLoading={cancellingAll}
                          size="sm"
                        >
                          Confirm
                        </Button>{' '}
                        <Button
                          variant="secondary"
                          onClick={() => setConfirmCancelAll(false)}
                          size="sm"
                        >
                          No
                        </Button>
                      </>
                    ) : (
                      <Button
                        variant="danger"
                        onClick={() => setConfirmCancelAll(true)}
                      >
                        Cancel All ({cancelAllCount})
                      </Button>
                    )}
                  </ToolbarItem>
                )}
                <ToolbarItem>
                  <Button
                    variant="secondary"
                    onClick={() => fetchBookings()}
                    icon={<SyncIcon />}
                  >
                    Refresh
                  </Button>
                </ToolbarItem>
                <ToolbarItem>
                  <Link to="/gpu-booking/help/getting-started" style={{ textDecoration: 'none' }}>
                    <Button
                      variant="secondary"
                      icon={<OutlinedQuestionCircleIcon />}
                    >
                      Help
                    </Button>
                  </Link>
                </ToolbarItem>
              </ToolbarContent>
            </Toolbar>
          </div>

          <div style={{ marginTop: '24px' }}>
            <ResourceSelector
              resources={gpuResources}
              selectedResources={selectedResources}
              onSelectionChange={setSelectedResources}
            />
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

          <GpuUsagePanel
            bookings={bookings}
            resources={gpuResources}
            selectedDate={selectedDates[0] || todayStr()}
          />

          <PreemptionBanner workloads={preemptedWorkloads} />

          <CalendarGrid
            viewYear={viewYear}
            viewMonth={viewMonth}
            selectedDates={selectedDates}
            bookings={bookings}
            bookingWindowDays={bookingWindowDays}
            gpuResources={gpuResources}
            canGoBack={canGoBack}
            canGoForward={canGoForward}
            onNavigateMonth={navigateMonth}
            onSelectDates={(dates) => { setSelectedDates(dates); setShowingMyBookings(false); }}
            onContextMenu={handleContextMenu}
            onGoToToday={() => {
              setViewMonth(now.getMonth());
              setViewYear(now.getFullYear());
              setSelectedDates([todayStr()]);
            }}
          />

          <div style={{ margin: '16px 0', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <div style={{ fontSize: '14px', color: 'var(--pf-t--global--text--color--regular)' }}>
              {selectedDates.length === 0
                ? 'Click a date to view'
                : `${selectedDates.length} date${selectedDates.length > 1 ? 's' : ''} selected`}
              <span style={{ marginLeft: '4px', opacity: 0.7 }}>(Ctrl+click to multi-select, Shift+click for range)</span>
            </div>
          </div>

          {gridDates.length === 0 ? (
            <EmptyState>
              <EmptyStateBody>
                Click a date in the calendar above to view bookings.
              </EmptyStateBody>
            </EmptyState>
          ) : (
            selectedResourceObjects.map((resource) => (
              <BookingGrid
                key={resource.type}
                resource={resource}
                dates={gridDates}
                bookings={bookings}
                activeReservations={activeReservations}
                currentUser={currentUser}
                reservingKey={reservingKey}
                confirmCancelId={confirmCancelId}
                onReserve={handleReserve}
                onCancel={handleCancel}
                onEdit={(b) => { setEditBooking(b); setShowBookingModal(true); }}
                onConfirmCancel={setConfirmCancelId}
              />
            ))
          )}
        </PageSection>

        {/* Context menu */}
        {contextMenu && (
          <div
            style={{
              position: 'fixed',
              left: contextMenu.x,
              top: contextMenu.y,
              zIndex: 1000,
              backgroundColor: 'var(--pf-t--global--background--color--primary--default)',
              borderRadius: '8px',
              boxShadow: '0 4px 16px rgba(0,0,0,0.3)',
              border: '1px solid var(--pf-t--global--border--color--default)',
              padding: '4px 0',
              minWidth: '160px',
            }}
          >
            <button
              onClick={() => {
                setContextMenu(null);
                setShowBookingModal(true);
              }}
              style={{
                width: '100%',
                textAlign: 'left',
                padding: '8px 16px',
                border: 'none',
                background: 'none',
                cursor: 'pointer',
                fontSize: '14px',
              }}
            >
              Book GPU
            </button>
          </div>
        )}

        {/* Booking modal */}
        {(showBookingModal && (selectedDates.length > 0 || editBooking)) && (
          <BookingModal
            isOpen={showBookingModal}
            startDate={editBooking?.date || [...selectedDates].sort()[0]}
            endDate={editBooking?.date || [...selectedDates].sort()[selectedDates.length - 1]}
            bookings={bookings}
            editBooking={editBooking || undefined}
            gpuResources={gpuResources}
            onClose={() => { setShowBookingModal(false); setEditBooking(null); }}
            onSubmit={handleBulkBooking}
          />
        )}
      </>
    </>
  );
};

export default BookingPage;
