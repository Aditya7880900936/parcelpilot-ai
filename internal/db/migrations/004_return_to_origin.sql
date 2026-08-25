ALTER TABLE orders
DROP CONSTRAINT orders_status_check;

ALTER TABLE orders
ADD CONSTRAINT orders_status_check
CHECK (
    status IN (
        'DRAFT',
        'BOOKED',
        'PICKED_UP',
        'RETURN_TO_ORIGIN',
        'DELIVERED',
        'CANCELLED'
    )
);
