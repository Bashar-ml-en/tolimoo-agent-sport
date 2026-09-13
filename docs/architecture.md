# Architecture

The single Go process exposes `net/http`, stores shared newsroom data in SQLite, and will later host scheduler and provider adapters. Expo Router is a thin client that consumes the stable contract through `src/services/api.ts`; `mock-api.ts` is an explicit local substitute.

Planned workflow: scheduler queues an assignment run → Exa researches developments → stories are deduplicated into shared history → OpenRouter produces a sourced French draft for Facebook/X → the editor reviews, edits, approves, or rejects → approval changes only internal state and never publishes externally.

Stories are shared across agents so repeated findings can be deduplicated. Drafts belong to an agent and run; messages belong to an agent conversation (one conversation per agent for this MVP).

Implemented now: schema bootstrap, seed data, health, agent listing with derived pending/running state, conversation reading, request logging/recovery, CORS, timeouts, body limit, graceful shutdown, and mobile screens with loading/empty/error states. Planned: real migration file execution, scheduler execution, Exa/OpenRouter clients, run/message/draft mutations, and publishing integrations.
