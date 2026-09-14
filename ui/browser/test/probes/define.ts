// Test material for test/build.test.ts, never imported by the app. A build without the production define leaves this expression
// unreplaced or replaces it with "development".
export const mode = process.env.NODE_ENV;
