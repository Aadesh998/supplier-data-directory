// One discovery cycle: claim N due jobs from the API, run each against its provider, report results.
// Jobs run sequentially with a short pause, which keeps us well within Overpass's fair-use policy.

import { runOverpass, runPlacesIds, type Job, type Result } from "./providers";

export type ApiCall = (path: string, body: unknown) => Promise<Response>;

export type RunnerConfig = {
  workerId: string;
  jobsPerRun: number;
  providers: string[];
  overpassUrl: string;
  userAgent: string;
  googleApiKey?: string;
  pauseMs?: number;
  fetchFn?: typeof fetch;
  sleep?: (ms: number) => Promise<void>;
};

export type CycleSummary = { claimed: number; completed: number; failed: number; results: number; errors: string[] };

const defaultSleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms));

export async function runDiscoveryCycle(api: ApiCall, cfg: RunnerConfig): Promise<CycleSummary> {
  const summary: CycleSummary = { claimed: 0, completed: 0, failed: 0, results: 0, errors: [] };
  const claimRes = await api("/api/v1/worker/jobs/claim", {
    worker_id: cfg.workerId,
    limit: cfg.jobsPerRun,
    providers: cfg.providers,
  });
  if (!claimRes.ok) {
    summary.errors.push(`claim failed: ${claimRes.status} ${await claimRes.text()}`);
    return summary;
  }
  const { jobs } = (await claimRes.json()) as { jobs: Job[] };
  summary.claimed = jobs.length;
  const sleep = cfg.sleep ?? defaultSleep;

  for (const [i, job] of jobs.entries()) {
    if (i > 0) await sleep(cfg.pauseMs ?? 2000);
    try {
      let results: Result[];
      if (job.provider === "osm_overpass") {
        results = await runOverpass(job, cfg.overpassUrl, cfg.userAgent, cfg.fetchFn);
      } else if (job.provider === "google_places_ids") {
        results = await runPlacesIds(job, cfg.googleApiKey, cfg.fetchFn);
      } else {
        throw new Error(`unsupported provider ${String(job.provider)}`);
      }
      const done = await api(`/api/v1/worker/jobs/${job.id}/complete`, { worker_id: cfg.workerId, results });
      if (!done.ok) throw new Error(`complete failed: ${done.status} ${await done.text()}`);
      summary.completed++;
      summary.results += results.length;
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      summary.failed++;
      summary.errors.push(`${job.job_key}: ${message}`);
      const res = await api(`/api/v1/worker/jobs/${job.id}/fail`, { worker_id: cfg.workerId, error: message });
      if (!res.ok) summary.errors.push(`${job.job_key}: could not record failure (${res.status})`);
    }
  }
  return summary;
}
