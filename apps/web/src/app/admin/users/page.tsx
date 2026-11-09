"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useApi, useAuth } from "@/components/admin/AuthProvider";
import { BoolBadge, EmptyState, ErrorState, LoadingState, PageHeader } from "@/components/admin/ui";
import { useToast } from "@/components/ui/Toast";
import { errorMessage } from "@/lib/api-error";
import { formatDate, humanize } from "@/lib/labels";
import type { AdminRole, AdminUser } from "@/lib/types";

const ROLE_HELP: Record<AdminRole, string> = {
  owner: "Everything, including source permissions, commercial exports, erasure and user management.",
  admin: "Edit, publish, import, merge and run discovery. Cannot manage users or change source permissions.",
  reviewer: "Review submissions and data. Read-mostly.",
};

const MIN_PASSWORD = 12;

export default function UsersPage() {
  const api = useApi();
  const { user: me } = useAuth();
  const toast = useToast();
  const qc = useQueryClient();
  const [form, setForm] = useState({ email: "", display_name: "", role: "reviewer" as Exclude<AdminRole, "owner">, password: "" });
  const [resetFor, setResetFor] = useState<AdminUser | null>(null);
  const [newPassword, setNewPassword] = useState("");

  const isOwner = me?.role === "owner";
  const q = useQuery({
    queryKey: ["admin", "users"],
    queryFn: () => api.get<{ data: AdminUser[] }>("/admin/users"),
    enabled: isOwner,
  });
  const refresh = () => void qc.invalidateQueries({ queryKey: ["admin", "users"] });

  const create = useMutation({
    mutationFn: () => api.post<AdminUser>("/admin/users", form),
    onSuccess: (u) => {
      toast.success(`Created ${u.role} ${u.email}. Share the password with them securely.`);
      setForm({ email: "", display_name: "", role: "reviewer", password: "" });
      refresh();
    },
    onError: (e) => toast.error(errorMessage(e)),
  });
  const update = useMutation({
    mutationFn: ({ id, patch }: { id: string; patch: Partial<Pick<AdminUser, "role" | "active" | "display_name">> }) =>
      api.patch<AdminUser>(`/admin/users/${id}`, patch),
    onSuccess: (u) => {
      toast.success(`Updated ${u.email}.`);
      refresh();
    },
    onError: (e) => toast.error(errorMessage(e)),
  });
  const reset = useMutation({
    mutationFn: ({ id, password }: { id: string; password: string }) => api.post(`/admin/users/${id}/password`, { password }),
    onSuccess: () => {
      toast.success(`Password reset for ${resetFor?.email}. Their sessions were signed out.`);
      setResetFor(null);
      setNewPassword("");
    },
    onError: (e) => toast.error(errorMessage(e)),
  });

  if (!isOwner) {
    return (
      <div className="space-y-4">
        <PageHeader title="Users" />
        <EmptyState>Only owners can manage admin users.</EmptyState>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Users"
        description="People who can sign in to the admin console. Give each person their own account with the lowest role that lets them do their work."
      />

      <form
        className="card space-y-4"
        onSubmit={(e) => {
          e.preventDefault();
          if (form.password.length < MIN_PASSWORD) return toast.error(`Password must be at least ${MIN_PASSWORD} characters.`);
          create.mutate();
        }}
      >
        <h2 className="text-lg font-semibold">Create a user</h2>
        <div className="grid gap-4 sm:grid-cols-2">
          <div>
            <label htmlFor="u-email" className="label">Email</label>
            <input id="u-email" type="email" required autoComplete="off" className="input" value={form.email}
              onChange={(e) => setForm({ ...form, email: e.target.value })} />
          </div>
          <div>
            <label htmlFor="u-name" className="label">Display name</label>
            <input id="u-name" className="input" maxLength={120} value={form.display_name}
              onChange={(e) => setForm({ ...form, display_name: e.target.value })} />
          </div>
          <div>
            <label htmlFor="u-role" className="label">Role</label>
            <select id="u-role" className="input" value={form.role}
              onChange={(e) => setForm({ ...form, role: e.target.value as "admin" | "reviewer" })}>
              <option value="reviewer">Reviewer</option>
              <option value="admin">Admin</option>
            </select>
            <p className="hint">{ROLE_HELP[form.role]} To make someone an owner, create them first, then change their role below.</p>
          </div>
          <div>
            <label htmlFor="u-pass" className="label">Initial password</label>
            <input id="u-pass" type="password" required minLength={MIN_PASSWORD} autoComplete="new-password" className="input"
              value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} />
            <p className="hint">At least {MIN_PASSWORD} characters. Send it to them over a private channel.</p>
          </div>
        </div>
        <button type="submit" className="btn-primary" disabled={create.isPending}>
          {create.isPending ? "Creating…" : "Create user"}
        </button>
      </form>

      {resetFor && (
        <form
          className="card space-y-3 border-accent-200"
          onSubmit={(e) => {
            e.preventDefault();
            if (newPassword.length < MIN_PASSWORD) return toast.error(`Password must be at least ${MIN_PASSWORD} characters.`);
            reset.mutate({ id: resetFor.id, password: newPassword });
          }}
        >
          <h2 className="text-lg font-semibold">Reset password for {resetFor.email}</h2>
          <label htmlFor="u-newpass" className="label">New password</label>
          <input id="u-newpass" type="password" minLength={MIN_PASSWORD} autoComplete="new-password" className="input max-w-md"
            value={newPassword} onChange={(e) => setNewPassword(e.target.value)} />
          <div className="flex gap-2">
            <button type="submit" className="btn-primary" disabled={reset.isPending}>Reset password</button>
            <button type="button" className="btn-secondary" onClick={() => { setResetFor(null); setNewPassword(""); }}>Cancel</button>
          </div>
        </form>
      )}

      {q.isLoading ? (
        <LoadingState />
      ) : q.isError ? (
        <ErrorState error={q.error} onRetry={() => void q.refetch()} />
      ) : !q.data?.data.length ? (
        <EmptyState>No users yet.</EmptyState>
      ) : (
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th scope="col">User</th>
                <th scope="col">Role</th>
                <th scope="col">Active</th>
                <th scope="col">Last sign-in</th>
                <th scope="col"><span className="sr-only">Actions</span></th>
              </tr>
            </thead>
            <tbody>
              {q.data.data.map((u) => {
                const self = u.id === me?.id;
                return (
                  <tr key={u.id}>
                    <td>
                      <p className="font-medium">{u.display_name || u.email}{self && <span className="ml-1 text-xs text-slate-600">(you)</span>}</p>
                      {u.display_name && <p className="text-xs text-slate-600">{u.email}</p>}
                    </td>
                    <td>
                      <label htmlFor={`role-${u.id}`} className="sr-only">Role for {u.email}</label>
                      <select id={`role-${u.id}`} className="input py-1" value={u.role} disabled={self || update.isPending}
                        onChange={(e) => {
                          const role = e.target.value as AdminRole;
                          if (window.confirm(`Change ${u.email} to ${humanize(role)}? They will be signed out.`)) update.mutate({ id: u.id, patch: { role } });
                        }}>
                        <option value="owner">Owner</option>
                        <option value="admin">Admin</option>
                        <option value="reviewer">Reviewer</option>
                      </select>
                    </td>
                    <td><BoolBadge value={u.active} yes="Active" no="Deactivated" /></td>
                    <td className="text-sm text-slate-700">{u.last_login_at ? formatDate(u.last_login_at) : "Never"}</td>
                    <td className="whitespace-nowrap text-right">
                      <button type="button" className="btn-ghost btn-sm" onClick={() => setResetFor(u)}>Reset password</button>
                      {!self && (
                        <button type="button" className={u.active ? "btn-ghost btn-sm text-red-700" : "btn-ghost btn-sm"} disabled={update.isPending}
                          onClick={() => {
                            if (!u.active || window.confirm(`Deactivate ${u.email}? They will be signed out immediately.`)) {
                              update.mutate({ id: u.id, patch: { active: !u.active } });
                            }
                          }}>
                          {u.active ? "Deactivate" : "Reactivate"}
                        </button>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
