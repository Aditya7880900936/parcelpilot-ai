CREATE UNIQUE INDEX IF NOT EXISTS idx_escalations_active_ticket
ON escalations(ticket_id)
WHERE status = 'created';