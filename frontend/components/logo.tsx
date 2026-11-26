import React, { useId } from "react";

interface LogoProps {
  variant?: "icon" | "horizontal" | "full";
  size?: "xs" | "sm" | "md" | "lg" | "xl";
  className?: string;
  iconClassName?: string;
  textClassName?: string;
  subtitle?: string;
  badge?: string;
  showText?: boolean;
}

const sizeConfig = {
  xs: {
    iconPx: 24,
    iconSize: "w-6 h-6",
    textSize: "text-base",
    subtextSize: "text-[10px]",
    badgeSize: "text-[9px] px-1.5 py-0.5",
  },
  sm: {
    iconPx: 32,
    iconSize: "w-8 h-8",
    textSize: "text-lg",
    subtextSize: "text-xs",
    badgeSize: "text-[10px] px-2 py-0.5",
  },
  md: {
    iconPx: 40,
    iconSize: "w-10 h-10",
    textSize: "text-xl",
    subtextSize: "text-xs",
    badgeSize: "text-xs px-2.5 py-0.5",
  },
  lg: {
    iconPx: 48,
    iconSize: "w-12 h-12",
    textSize: "text-2xl",
    subtextSize: "text-sm",
    badgeSize: "text-xs px-3 py-1",
  },
  xl: {
    iconPx: 64,
    iconSize: "w-16 h-16",
    textSize: "text-3xl",
    subtextSize: "text-base",
    badgeSize: "text-sm px-3.5 py-1",
  },
};

export function LogoIcon({
  className = "w-8 h-8",
  size = 32,
}: {
  className?: string;
  size?: number;
}) {
  const uid = useId();
  const bgId = `${uid}-bg`;
  const borderId = `${uid}-border`;
  const frameId = `${uid}-frame`;
  const boltId = `${uid}-bolt`;
  const innerId = `${uid}-inner`;
  const glowId = `${uid}-glow`;

  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 512 512"
      width={size}
      height={size}
      fill="none"
      className={`shrink-0 ${className}`}
      role="img"
      aria-label="NextDeploy Icon"
    >
      <defs>
        <linearGradient id={bgId} x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#0b1329" />
          <stop offset="50%" stopColor="#050a17" />
          <stop offset="100%" stopColor="var(--color-surface-950)" />
        </linearGradient>
        <linearGradient id={borderId} x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="var(--color-brand-400)" stopOpacity="0.7" />
          <stop offset="45%" stopColor="var(--color-cyan-500)" stopOpacity="0.4" />
          <stop offset="100%" stopColor="var(--color-surface-800)" stopOpacity="0.8" />
        </linearGradient>
        <linearGradient id={frameId} x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="var(--color-brand-500)" />
          <stop offset="100%" stopColor="var(--color-brand-600)" />
        </linearGradient>
        <linearGradient id={boltId} x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="var(--color-brand-200)" />
          <stop offset="25%" stopColor="var(--color-brand-400)" />
          <stop offset="70%" stopColor="var(--color-brand-500)" />
          <stop offset="100%" stopColor="var(--color-cyan-500)" />
        </linearGradient>
        <linearGradient id={innerId} x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="var(--color-surface-900)" stopOpacity="0.95" />
          <stop offset="100%" stopColor="var(--color-surface-950)" stopOpacity="0.98" />
        </linearGradient>
        <filter id={glowId} x="-20%" y="-20%" width="140%" height="140%">
          <feGaussianBlur stdDeviation="8" result="blur" />
          <feComposite in="SourceGraphic" in2="blur" operator="over" />
        </filter>
      </defs>

      {/* Outer Squircle Chassis */}
      <rect
        x="16"
        y="16"
        width="480"
        height="480"
        rx="112"
        fill={`url(#${bgId})`}
        stroke={`url(#${borderId})`}
        strokeWidth="10"
      />

      {/* Inner Server Box */}
      <rect
        x="96"
        y="96"
        width="320"
        height="320"
        rx="56"
        fill={`url(#${innerId})`}
        stroke={`url(#${frameId})`}
        strokeWidth="16"
      />

      {/* Server Status LEDs */}
      <circle cx="148" cy="148" r="10" fill="var(--color-brand-400)" />
      <circle cx="180" cy="148" r="10" fill="var(--color-sky-400)" />
      <rect x="300" y="143" width="64" height="10" rx="5" fill="var(--color-surface-700)" />

      {/* High-Energy Deploy Lightning Bolt */}
      <path
        d="M275 185 L215 270 L250 270 L220 360 L305 250 L262 250 Z"
        fill={`url(#${boltId})`}
        filter={`url(#${glowId})`}
      />
    </svg>
  );
}

export default function Logo({
  variant = "horizontal",
  size = "sm",
  className = "",
  iconClassName = "",
  textClassName = "",
  subtitle,
  badge,
  showText = true,
}: LogoProps) {
  const cfg = sizeConfig[size] || sizeConfig.sm;

  if (variant === "icon" || !showText) {
    return (
      <div className={`inline-flex items-center ${className}`}>
        <LogoIcon className={`${cfg.iconSize} ${iconClassName}`} size={cfg.iconPx} />
      </div>
    );
  }

  return (
    <div className={`inline-flex items-center gap-2.5 ${className}`}>
      <LogoIcon className={`${cfg.iconSize} ${iconClassName}`} size={cfg.iconPx} />
      
      <div className="flex flex-col">
        <div className="flex items-center gap-2">
          <span
            className={`font-extrabold tracking-tight text-white ${cfg.textSize} ${textClassName}`}
          >
            Next
            <span className="bg-gradient-to-r from-brand-400 via-accent-400 to-cyan-400 bg-clip-text text-transparent">
              Deploy
            </span>
          </span>

          {badge && (
            <span
              className={`rounded-full font-mono font-semibold bg-brand-500/10 text-brand-400 border border-brand-500/20 ${cfg.badgeSize}`}
            >
              {badge}
            </span>
          )}
        </div>

        {(variant === "full" || subtitle) && (
          <span
            className={`font-mono text-muted-400 tracking-wider uppercase ${cfg.subtextSize}`}
          >
            {subtitle || "Self-Hosted Next.js Engine"}
          </span>
        )}
      </div>
    </div>
  );
}
