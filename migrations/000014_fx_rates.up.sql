CREATE TABLE fx_rates (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    from_currency   TEXT NOT NULL,
    to_currency     TEXT NOT NULL,
    mid_rate        DOUBLE PRECISION NOT NULL CHECK (mid_rate > 0),
    bid_rate        DOUBLE PRECISION NOT NULL CHECK (bid_rate > 0),
    ask_rate        DOUBLE PRECISION NOT NULL CHECK (ask_rate > 0),
    spread_percent  DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (spread_percent >= 0),
    source          TEXT NOT NULL DEFAULT 'api'
                        CHECK (source IN ('nbe_indicative','manual','api')),
    fetched_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Fast lookup: latest rate for a currency pair
CREATE INDEX idx_fx_rates_pair_latest
    ON fx_rates (from_currency, to_currency, created_at DESC);

-- Fast lookup: latest rate by source
CREATE INDEX idx_fx_rates_source
    ON fx_rates (source, created_at DESC);

-- No seed data. The table starts empty.
-- Rates are populated by the hourly cron job (ChainedRateSource) or admin
-- manual override. Until the first cron run completes, the public rate API
-- and convert service will return 503 "exchange rates not yet available".
-- On a fresh deploy, trigger the first fetch manually via:
--   POST /admin/v1/fx/rates/refresh
