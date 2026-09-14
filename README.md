# RS3 Market Backend

Microservices for RuneScape 3 market analysis.

Services:
- hiscore-service  :8081  — on-demand RS3 hiscores
- recipe-service   :8082  — crafting recipe scraper + tree
- ge-price-service :8083  — Weirdgloop GE poller
- calc-service     :8084  — ROI / GP/h / XP/h / GP/XP
- realtime-service :8085  — WebSocket price push + chat

Run: `docker compose up --build`
