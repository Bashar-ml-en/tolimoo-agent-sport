export type League = { id: string; name: string; country: string; badgeURL: string; agentID?: string };

// Competition badges are served by API-Sports' public football badge CDN.
// They remain remote so the app does not redistribute league marks locally.
export const leagues: League[] = [
  { id:'champions-league', name:'UEFA Champions League', country:'Europe', badgeURL:'https://media.api-sports.io/football/leagues/2.png' },
  { id:'premier-league', name:'Premier League', country:'England', badgeURL:'https://media.api-sports.io/football/leagues/39.png', agentID:'premier_league' },
  { id:'laliga', name:'LaLiga', country:'Spain', badgeURL:'https://media.api-sports.io/football/leagues/140.png' },
  { id:'bundesliga', name:'Bundesliga', country:'Germany', badgeURL:'https://media.api-sports.io/football/leagues/78.png', agentID:'bundesliga' },
  { id:'serie-a', name:'Serie A', country:'Italy', badgeURL:'https://media.api-sports.io/football/leagues/135.png' },
  { id:'ligue-1', name:'Ligue 1', country:'France', badgeURL:'https://media.api-sports.io/football/leagues/61.png' },
];
