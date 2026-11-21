CREATE TABLE `Sales``Table` (
  `tenant` INT NOT NULL COMMENT '',
  `id` INT NOT NULL AUTO_INCREMENT,
  `display``name` VARCHAR(20) NULL DEFAULT 'line\nnext\\path' COMMENT '名称 "x"\\line\n下一行',
  `status` CHAR(1) NULL DEFAULT '',
  `created` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `note` TEXT NULL DEFAULT ('hello'),
  `payload` JSON NULL DEFAULT (JSON_ARRAY()),
  `amount` DECIMAL(20,2) NULL DEFAULT 12.30,
  `deleted_at` DATETIME(6) NULL DEFAULT NULL,
  PRIMARY KEY (`id`, `tenant`),
  UNIQUE KEY `ix-name` (`display``name`, `tenant`),
  KEY `ix_note_desc` (`note`(16) DESC) COMMENT 'note suffix'
) ENGINE=InnoDB COMMENT='客户表';
