'use client';

import React, { useState } from 'react';
import { BlogPictureData } from '../../lib/types';

interface BlogPictureProps {
  data: BlogPictureData;
}

export const BlogPicture: React.FC<BlogPictureProps> = ({ data }) => {
  const [isZoomed, setIsZoomed] = useState(false);
  const imageSrc = data.src || data.lightSrc || data.darkSrc || '';
  const altText = data.alt || 'Blog illustration';

  return (
    <figure className="my-8 flex flex-col items-center">
      <div
        className="w-full overflow-hidden rounded-xl border border-slate-200/80 bg-white p-2 shadow-sm transition-all hover:shadow-md cursor-zoom-in"
        style={{
          maxWidth: data.maxWidth || '100%',
          aspectRatio: data.aspectRatio ? data.aspectRatio.replace('/', ' / ') : undefined,
        }}
        onClick={() => setIsZoomed(true)}
      >
        <img
          src={imageSrc}
          alt={altText}
          className="h-full w-full object-contain rounded-lg transition-transform hover:scale-[1.01]"
          loading="lazy"
        />
      </div>

      {data.caption && (
        <figcaption className="mt-2.5 text-center text-xs text-slate-500 font-medium tracking-tight">
          {data.caption}
        </figcaption>
      )}

      {/* Lightbox / Zoom Modal */}
      {isZoomed && (
        <div
          role="dialog"
          aria-modal="true"
          className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/80 backdrop-blur-sm p-4 animate-in fade-in"
          onClick={() => setIsZoomed(false)}
        >
          <div className="relative max-h-[90vh] max-w-[90vw] overflow-hidden rounded-xl bg-white p-4 shadow-2xl">
            <button
              type="button"
              onClick={() => setIsZoomed(false)}
              className="absolute top-2 right-2 rounded-full bg-slate-100 p-1.5 text-slate-600 hover:bg-slate-200"
              aria-label="Close zoomed image"
            >
              <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
            <img
              src={imageSrc}
              alt={altText}
              className="max-h-[80vh] w-auto max-w-full object-contain"
            />
            {data.caption && (
              <p className="mt-3 text-center text-xs text-slate-600">{data.caption}</p>
            )}
          </div>
        </div>
      )}
    </figure>
  );
};
