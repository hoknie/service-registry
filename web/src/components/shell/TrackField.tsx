export function TrackField({ className }: { className?: string }) {
  return (
    <svg aria-hidden="true" viewBox="0 0 1200 800" preserveAspectRatio="xMidYMid slice" className={className}>
      <defs>
        <radialGradient id="track-fade" cx="50%" cy="50%" r="60%">
          <stop offset="0%" stopColor="white" stopOpacity="1" />
          <stop offset="100%" stopColor="white" stopOpacity="0" />
        </radialGradient>
        <mask id="track-mask">
          <rect width="1200" height="800" fill="url(#track-fade)" />
        </mask>
      </defs>
      <g mask="url(#track-mask)" fill="none" stroke="currentColor" strokeWidth="1.5">
        <path d="M-20 560 H420 C560 560 620 470 760 470 H1220" />
        <path d="M-20 590 H420 C560 590 620 500 760 500 H1220" opacity="0.7" />
        <path d="M420 560 C560 560 640 650 800 650 H1220" opacity="0.8" />
        <path d="M-20 250 H300 C440 250 500 330 640 330 H1220" opacity="0.6" />
        <path d="M300 250 C430 250 520 160 680 160 H1220" opacity="0.5" />
        <path d="M-20 700 H1220" opacity="0.35" strokeDasharray="10 14" />
        <path d="M-20 100 H1220" opacity="0.3" strokeDasharray="10 14" />
      </g>
      <g mask="url(#track-mask)">
        <circle cx="760" cy="470" r="5" fill="var(--signal)" />
        <circle cx="640" cy="330" r="4" fill="var(--amber)" opacity="0.8" />
        <circle cx="800" cy="650" r="4" fill="var(--cobalt)" opacity="0.8" />
      </g>
    </svg>
  );
}
