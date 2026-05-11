import { useState } from 'react';

export interface GalleryImage {
  src: string;
  alt?: string;
}

interface ImageGalleryModalProps {
  images: GalleryImage[];
  initialIndex?: number;
  isOpen: boolean;
  onClose: () => void;
}

export const ImageGalleryModal: React.FC<ImageGalleryModalProps> = ({
  images,
  initialIndex = 0,
  isOpen,
  onClose,
}) => {
  const [currentIndex, setCurrentIndex] = useState(initialIndex);

  if (!isOpen || images.length === 0) return null;

  const current = images[currentIndex];

  return (
    <div
      style={{
        position: 'fixed', inset: 0, zIndex: 9999,
        background: 'rgba(0,0,0,0.85)', display: 'flex',
        alignItems: 'center', justifyContent: 'center',
      }}
      onClick={onClose}
    >
      <div onClick={(e) => e.stopPropagation()} style={{ position: 'relative', maxWidth: '90vw', maxHeight: '90vh' }}>
        <img
          src={current.src}
          alt={current.alt || ''}
          style={{ maxWidth: '90vw', maxHeight: '85vh', objectFit: 'contain', borderRadius: '8px' }}
        />
        {images.length > 1 && (
          <div style={{ display: 'flex', justifyContent: 'center', gap: '1rem', marginTop: '1rem' }}>
            <button
              onClick={() => setCurrentIndex((i) => (i > 0 ? i - 1 : images.length - 1))}
              style={{ color: 'white', background: 'rgba(255,255,255,0.2)', border: 'none', borderRadius: '50%', width: 36, height: 36, cursor: 'pointer' }}
            >
              &larr;
            </button>
            <span style={{ color: 'white', alignSelf: 'center' }}>{currentIndex + 1} / {images.length}</span>
            <button
              onClick={() => setCurrentIndex((i) => (i < images.length - 1 ? i + 1 : 0))}
              style={{ color: 'white', background: 'rgba(255,255,255,0.2)', border: 'none', borderRadius: '50%', width: 36, height: 36, cursor: 'pointer' }}
            >
              &rarr;
            </button>
          </div>
        )}
      </div>
    </div>
  );
};
