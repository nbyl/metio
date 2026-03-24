import { initializeApp, type FirebaseApp } from 'firebase/app';
import { type Firestore, initializeFirestore } from 'firebase/firestore';
import { getAuth, signInWithCustomToken, type Auth } from 'firebase/auth';
import type { AppConfig, FirebaseTokenResponse } from '../types/firestore';

let app: FirebaseApp | null = null;
let db: Firestore | null = null;
let auth: Auth | null = null;
let config: AppConfig | null = null;

/**
 * Fetches the application configuration from the backend
 */
export async function fetchConfig(): Promise<AppConfig> {
  if (config) return config;

  const response = await fetch('/api/config');
  if (!response.ok) {
    throw new Error('Failed to fetch config');
  }
  config = (await response.json()) as AppConfig;
  return config;
}

/**
 * Initializes Firebase and authenticates with a custom token from the backend
 */
export async function initializeFirebase(): Promise<{
  db: Firestore;
  auth: Auth;
  config: AppConfig;
}> {
  if (db && auth && config) {
    return { db, auth, config };
  }

  // Fetch config from backend
  const appConfig = await fetchConfig();

  // Initialize Firebase app
  app = initializeApp({
    projectId: appConfig.gcpProject,
  });

  // Initialize Firestore with the specific database
  db = initializeFirestore(app, {}, appConfig.firestoreDatabase);

  // Initialize Auth
  auth = getAuth(app);

  // Get Firebase custom token from backend and sign in
  const tokenResponse = await fetch('/api/auth/firebase-token');
  if (!tokenResponse.ok) {
    throw new Error('Failed to get Firebase token');
  }
  const { token }: FirebaseTokenResponse = await tokenResponse.json();

  // Sign in with the custom token
  await signInWithCustomToken(auth, token);

  return { db, auth, config: appConfig };
}

/**
 * Returns the Firestore instance (must call initializeFirebase first)
 */
export function getFirestoreDb(): Firestore {
  if (!db) {
    throw new Error('Firebase not initialized. Call initializeFirebase() first.');
  }
  return db;
}

/**
 * Returns the cached config (must call initializeFirebase or fetchConfig first)
 */
export function getConfig(): AppConfig {
  if (!config) {
    throw new Error('Config not loaded. Call initializeFirebase() first.');
  }
  return config;
}
