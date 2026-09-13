curl -sS -X POST http://localhost:8080/api/v1/agents \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "laliga_reporter",
    "assignment": "LaLiga news",
    "language": "fr",
    "platforms": ["facebook", "x"],
    "enabled": true,
    "researchIntervalSeconds": 60
  }'