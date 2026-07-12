import React from 'react';

interface SavePdfButtonProps {
    onClick: () => void;
    busy?: boolean;
    className?: string;
    text?: string;
}

const SavePdfButton: React.FC<SavePdfButtonProps> = ({ onClick, busy, className, text = 'Save PDF' }) => {
    return (
        <div
            onClick={onClick}
            className={(className ?? 'fileButton') + (busy ? ' loading' : '')}
        >
            {busy ? 'Generating...' : text}
        </div>
    );
};

export default SavePdfButton;