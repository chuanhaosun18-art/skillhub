import texture from "../../assets/skillx-glass-data-flow.png";

export function IceFlowBackground({ variant = "subtle" }: { variant?: "hero" | "subtle" }) {
  return (
    <div className={`ice-flow ice-flow--${variant}`} aria-hidden="true">
      <img className="ice-flow__texture" src={texture} alt="" />
      <svg
        className="ice-flow__vector"
        viewBox="0 0 1600 900"
        preserveAspectRatio="none"
        xmlns="http://www.w3.org/2000/svg"
      >
        <defs>
          <linearGradient id="ice-sheet-a" x1="0" y1="0" x2="1" y2="1">
            <stop offset="0" stopColor="#ffffff" stopOpacity="0.7" />
            <stop offset="0.42" stopColor="#c7f2ff" stopOpacity="0.18" />
            <stop offset="1" stopColor="#75d8f4" stopOpacity="0.1" />
          </linearGradient>
          <linearGradient id="ice-sheet-b" x1="1" y1="0" x2="0" y2="1">
            <stop offset="0" stopColor="#f9feff" stopOpacity="0.82" />
            <stop offset="0.58" stopColor="#9bddff" stopOpacity="0.12" />
            <stop offset="1" stopColor="#3f87d9" stopOpacity="0.08" />
          </linearGradient>
          <filter id="ice-noise" x="-20%" y="-20%" width="140%" height="140%">
            <feTurbulence type="fractalNoise" baseFrequency="0.004 0.012" numOctaves="3" seed="19" result="noise" />
            <feDisplacementMap in="SourceGraphic" in2="noise" scale="54" xChannelSelector="R" yChannelSelector="B" />
            <feGaussianBlur stdDeviation="7" />
          </filter>
        </defs>
        <g filter="url(#ice-noise)">
          <path className="ice-flow__sheet ice-flow__sheet--a" d="M-120 580C230 390 430 550 760 365C1070 190 1260 295 1730 80V970H-120Z" fill="url(#ice-sheet-a)" />
          <path className="ice-flow__sheet ice-flow__sheet--b" d="M-180 745C235 515 500 720 835 505C1135 312 1390 420 1740 245V980H-180Z" fill="url(#ice-sheet-b)" />
        </g>
      </svg>
      <div className="ice-flow__scan" />
      <div className="ice-flow__glow" />
    </div>
  );
}
