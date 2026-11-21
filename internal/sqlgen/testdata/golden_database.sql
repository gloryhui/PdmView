CREATE DATABASE IF NOT EXISTS `pdmview_golden` CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;

USE `pdmview_golden`;

CREATE TABLE `alpha` (
  `id` INT NOT NULL,
  `zeta_id` INT NULL,
  `parent_id` INT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB;

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

CREATE TABLE `zeta` (
  `id` INT NOT NULL,
  `alpha_id` INT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB;

ALTER TABLE `alpha` ADD CONSTRAINT `fk_alpha_self` FOREIGN KEY (`parent_id`) REFERENCES `alpha` (`id`);

ALTER TABLE `alpha` ADD CONSTRAINT `fk_alpha_zeta` FOREIGN KEY (`zeta_id`) REFERENCES `zeta` (`id`);

ALTER TABLE `zeta` ADD CONSTRAINT `fk_zeta_alpha` FOREIGN KEY (`alpha_id`) REFERENCES `alpha` (`id`);
