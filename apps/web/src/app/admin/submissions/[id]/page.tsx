"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useApi } from "@/components/admin/AuthProvider";
import { SubmissionReview } from "@/components/admin/SubmissionReview";
import { ErrorState, LoadingState, PageHeader } from "@/components/admin/ui";
import type { SubmissionDetail } from "@/lib/types";

export default function SubmissionDetailPage() {
  const { id } = useParams<{ id: string }>();
  const api = useApi();
  const q = useQuery({ queryKey: ["admin", "submission", id], queryFn: () => api.get<SubmissionDetail>(`/admin/submissions/${id}`), enabled: !!id });
  return (
    <div className="space-y-4">
      <p className="text-sm">
        <Link href="/admin/submissions" className="link">
          ← All submissions
        </Link>
      </p>
      {q.isLoading ? (
        <LoadingState />
      ) : q.error ? (
        <ErrorState error={q.error} onRetry={() => q.refetch()} />
      ) : q.data ? (
        <>
          <PageHeader title={q.data.company_name} description="Review the submitted details and permissions. Submitter details are private." />
          <SubmissionReview submission={q.data} />
        </>
      ) : null}
    </div>
  );
}
