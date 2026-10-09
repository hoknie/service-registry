export function BrandMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 28 28" aria-hidden="true" className={className}>
      <rect width="28" height="28" rx="7" fill="var(--primary)" />
      <path d="M9 22V6" stroke="var(--primary-ink)" strokeWidth="2.4" strokeLinecap="round" fill="none" />
      <path d="M9 17.5c0-5 3.2-8.2 9.2-9.6" stroke="var(--primary-ink)" strokeWidth="2.4" strokeLinecap="round" fill="none" opacity="0.72" />
      <circle cx="19.6" cy="7.6" r="2.6" fill="var(--signal)" />
    </svg>
  );
}
