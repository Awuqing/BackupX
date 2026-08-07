import type { SVGProps } from 'react'

export function BackupServerIllustration(props: SVGProps<SVGSVGElement>) {
  return (
    <svg
      width="320"
      height="320"
      viewBox="0 0 320 320"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
      focusable="false"
      {...props}
    >
      <circle cx="160" cy="160" r="120" fill="white" fillOpacity="0.05">
        <animate attributeName="r" values="115;125;115" dur="4s" repeatCount="indefinite" />
        <animate attributeName="fill-opacity" values="0.03;0.08;0.03" dur="4s" repeatCount="indefinite" />
      </circle>
      <circle cx="160" cy="160" r="80" fill="white" fillOpacity="0.1">
        <animate attributeName="r" values="75;85;75" dur="3s" repeatCount="indefinite" />
      </circle>

      <g>
        <animateTransform attributeName="transform" type="translate" values="0,0; 0,-8; 0,0" dur="5s" repeatCount="indefinite" />
        <path d="M120 120C120 111.163 137.909 104 160 104C182.091 104 200 111.163 200 120V144C200 152.837 182.091 160 160 160C137.909 160 120 152.837 120 144V120Z" fill="white" fillOpacity="0.95" />
        <ellipse cx="160" cy="120" rx="40" ry="16" fill="white" />
        <path d="M120 152C120 143.163 137.909 136 160 136C182.091 136 200 143.163 200 152V176C200 184.837 182.091 192 160 192C137.909 192 120 184.837 120 176V152Z" fill="white" fillOpacity="0.75" />
        <ellipse cx="160" cy="152" rx="40" ry="16" fill="white" fillOpacity="0.9" />
        <path d="M120 184C120 175.163 137.909 168 160 168C182.091 168 200 175.163 200 184V208C200 216.837 182.091 224 160 224C137.909 224 120 216.837 120 208V184Z" fill="white" fillOpacity="0.5" />
        <ellipse cx="160" cy="184" rx="40" ry="16" fill="white" fillOpacity="0.6" />

        <g fill="var(--color-primary-6, #165dff)">
          <circle cx="140" cy="120" r="4">
            <animate attributeName="opacity" values="0.3;1;0.3" dur="2s" begin="0s" repeatCount="indefinite" />
          </circle>
          <circle cx="140" cy="152" r="4">
            <animate attributeName="opacity" values="0.3;1;0.3" dur="2s" begin="0.6s" repeatCount="indefinite" />
          </circle>
          <circle cx="140" cy="184" r="4">
            <animate attributeName="opacity" values="0.3;1;0.3" dur="2s" begin="1.2s" repeatCount="indefinite" />
          </circle>
        </g>

        <path d="M160 120V152V184" stroke="var(--color-primary-6, #165dff)" strokeWidth="2" strokeDasharray="4 4" opacity="0.6">
          <animate attributeName="stroke-dashoffset" from="16" to="0" dur="1s" repeatCount="indefinite" />
        </path>
      </g>
    </svg>
  )
}
